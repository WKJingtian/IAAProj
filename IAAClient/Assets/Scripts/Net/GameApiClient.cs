using System;
using System.Collections;
using UnityEngine;
using UnityEngine.Networking;

public class GameApiClient
{
    [Serializable]
    public class DebugValResponse
    {
        public string openid;
        public int debug_val;
        public string errMsg;
    }

    private readonly string _gatewayBaseUrl;

    public GameApiClient(string gatewayBaseUrl)
    {
        _gatewayBaseUrl = NormalizeBaseUrl(gatewayBaseUrl);
    }

    public IEnumerator GetDebugVal(string token, Action<DebugValResponse> onSuccess, Action<string> onFailed)
    {
        yield return SendAuthorizedRequest("GET", "/debug_val", token, onSuccess, onFailed);
    }

    public IEnumerator IncrementDebugVal(string token, Action<DebugValResponse> onSuccess, Action<string> onFailed)
    {
        yield return SendAuthorizedRequest("POST", "/debug_val_inc", token, onSuccess, onFailed);
    }

    private IEnumerator SendAuthorizedRequest(string method, string path, string token, Action<DebugValResponse> onSuccess, Action<string> onFailed)
    {
        if (string.IsNullOrEmpty(token))
        {
            onFailed?.Invoke("token cannot be empty");
            yield break;
        }

        string url = _gatewayBaseUrl + path;
        using (UnityWebRequest request = new UnityWebRequest(url, method))
        {
            request.downloadHandler = new DownloadHandlerBuffer();
            request.uploadHandler = new UploadHandlerRaw(new byte[0]);
            request.SetRequestHeader("Authorization", "Bearer " + token);
            request.SetRequestHeader("Content-Type", "application/json");

            yield return request.SendWebRequest();

            if (request.result != UnityWebRequest.Result.Success)
            {
                string responseBody = request.downloadHandler != null ? request.downloadHandler.text : "";
                onFailed?.Invoke("request failed: " + request.error + ", body=" + responseBody);
                yield break;
            }

            string body = request.downloadHandler.text;
            DebugValResponse response;
            try
            {
                response = JsonUtility.FromJson<DebugValResponse>(body);
            }
            catch (Exception ex)
            {
                onFailed?.Invoke("parse debug response failed: " + ex.Message + ", body=" + body);
                yield break;
            }

            if (response == null)
            {
                onFailed?.Invoke("debug response is null");
                yield break;
            }
            if (!string.IsNullOrEmpty(response.errMsg))
            {
                onFailed?.Invoke(response.errMsg);
                yield break;
            }

            onSuccess?.Invoke(response);
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
