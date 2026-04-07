using System;
using System.Threading.Tasks;
using Newtonsoft.Json;
using UnityEngine;
using UnityEngine.Networking;

public class GatewayAuthClient
{
    private readonly string _gatewayBaseUrl;
    private readonly string _wxAppId;

    [Serializable]
    private class LoginResponse : NetResponseBase
    {
        [JsonProperty("openid",
            Required = Required.Default,
            NullValueHandling = NullValueHandling.Ignore)]
        public string OpenID;

        [JsonProperty("token",
            Required = Required.Default,
            NullValueHandling = NullValueHandling.Ignore)]
        public string Token;
    }

    public GatewayAuthClient(string gatewayBaseUrl, string wxAppId)
    {
        _gatewayBaseUrl = gatewayBaseUrl.TrimEnd('/');
        _wxAppId = wxAppId;
    }

    // send code to our server, returns the user-specific open-ID and a JWT for future authentication on our server
    public async Task<AuthSession> LoginWithCodeAsync(string code)
    {
        string loginUrl = _gatewayBaseUrl + Consts.GatewayLoginPath;
        WWWForm form = new WWWForm();
        form.AddField("code", code);
        form.AddField("appid", _wxAppId);

        using (UnityWebRequest request = UnityWebRequest.Post(loginUrl, form))
        {
            UnityWebRequestAsyncOperation operation = request.SendWebRequest();
            while (!operation.isDone)
            {
                await Task.Yield();
            }

            string body = request.downloadHandler.text;
            LoginResponse response;
            try
            {
                response = JsonConvert.DeserializeObject<LoginResponse>(body);
            }
            catch (Exception ex)
            {
                throw new AppErrorException(ErrorCode.CLIENT_RESPONSE_PARSE_FAILED, ex);
            }

            if (response == null)
            {
                throw new AppErrorException(ErrorCode.CLIENT_RESPONSE_INVALID);
            }
            if (response.ErrMsg != ErrorCode.OK)
            {
                throw new AppErrorException(response.ErrMsg);
            }

            if (request.result != UnityWebRequest.Result.Success)
            {
                throw new AppErrorException(ErrorCode.CLIENT_REQUEST_FAILED);
            }

            AuthSession session = new AuthSession
            {
                openid = response.OpenID,
                token = response.Token
            };

            if (!session.IsValid)
            {
                throw new AppErrorException(ErrorCode.CLIENT_LOGIN_RESPONSE_INVALID);
            }

            return session;
        }
    }
}
