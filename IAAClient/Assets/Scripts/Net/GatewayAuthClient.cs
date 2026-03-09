using System;
using System.Collections;
using UnityEngine;
using UnityEngine.Networking;

public class GatewayAuthClient
{
    private readonly string _gatewayBaseUrl;
    private readonly string _wxAppId;

    [Serializable]
    private class LoginResponse
    {
        public string openid;
        public string token;
        public string errMsg;
    }

    public GatewayAuthClient(string gatewayBaseUrl, string wxAppId)
    {
        _gatewayBaseUrl = NormalizeBaseUrl(gatewayBaseUrl);
        _wxAppId = wxAppId;
    }

    public IEnumerator LoginWithCode(string code, Action<AuthSession> onSuccess, Action<string> onFailed)
    {
        if (string.IsNullOrEmpty(code))
        {
            onFailed?.Invoke("login code cannot be empty");
            yield break;
        }
        if (string.IsNullOrEmpty(_wxAppId))
        {
            onFailed?.Invoke("WX_APP_ID cannot be empty");
            yield break;
        }

        string loginUrl = _gatewayBaseUrl + "/login";
        WWWForm form = new WWWForm();
        form.AddField("code", code);
        form.AddField("appid", _wxAppId);

        using (UnityWebRequest request = UnityWebRequest.Post(loginUrl, form))
        {
            yield return request.SendWebRequest();

            if (request.result != UnityWebRequest.Result.Success)
            {
                onFailed?.Invoke("gateway login request failed: " + request.error);
                yield break;
            }

            string body = request.downloadHandler.text;
            LoginResponse response;
            try
            {
                response = JsonUtility.FromJson<LoginResponse>(body);
            }
            catch (Exception ex)
            {
                onFailed?.Invoke("parse login response failed: " + ex.Message + ", body=" + body);
                yield break;
            }

            if (response == null)
            {
                onFailed?.Invoke("login response is null");
                yield break;
            }

            if (!string.IsNullOrEmpty(response.errMsg))
            {
                onFailed?.Invoke(response.errMsg);
                yield break;
            }

            AuthSession session = new AuthSession
            {
                openid = response.openid,
                token = response.token
            };

            if (!session.IsValid)
            {
                onFailed?.Invoke("login response missing openid or token");
                yield break;
            }

            onSuccess?.Invoke(session);
        }
    }

    private static string NormalizeBaseUrl(string baseUrl)
    {
        if (string.IsNullOrEmpty(baseUrl))
        {
            return "";
        }
        return baseUrl.TrimEnd('/');
    }
}
