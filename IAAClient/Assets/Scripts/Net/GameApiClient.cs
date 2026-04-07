using System;
using System.Threading.Tasks;
using Newtonsoft.Json;
using UnityEngine.Networking;

public class GameApiClient
{
    private readonly string _gatewayBaseUrl;

    public GameApiClient(string gatewayBaseUrl)
    {
        _gatewayBaseUrl = gatewayBaseUrl.TrimEnd('/');
    }

    public async Task<TResponse> SendAuthorizedRequestAsync<TResponse>(NetAPI<TResponse> api, string token)
        where TResponse : NetResponseBase
    {
        return await SendAuthorizedRequestAsync(api, token, null);
    }

    public async Task<TResponse> SendAuthorizedRequestAsync<TResponse>(NetAPI<TResponse> api, string token, object requestBody)
        where TResponse : NetResponseBase
    {
        string url = _gatewayBaseUrl + api.requestPath;
        using (UnityWebRequest request = new UnityWebRequest(url, api.Method))
        {
            request.downloadHandler = new DownloadHandlerBuffer();
            byte[] payloadBytes = BuildRequestBody(requestBody);
            request.uploadHandler = new UploadHandlerRaw(payloadBytes);
            request.SetRequestHeader("Authorization", "Bearer " + token);
            request.SetRequestHeader("Content-Type", "application/json");

            UnityWebRequestAsyncOperation operation = request.SendWebRequest();
            while (!operation.isDone)
            {
                await Task.Yield();
            }

            string body = request.downloadHandler.text;
            TResponse response;
            try
            {
                response = JsonConvert.DeserializeObject<TResponse>(body);
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

            return response;
        }
    }

    private static byte[] BuildRequestBody(object requestBody)
    {
        if (requestBody == null)
        {
            return Array.Empty<byte>();
        }

        string json = JsonConvert.SerializeObject(requestBody);
        return System.Text.Encoding.UTF8.GetBytes(json);
    }
}
