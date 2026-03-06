using UnityEngine;
using WeChatWASM;
using System.Collections;
using UnityEngine.Networking;
using System;
using System.Threading.Tasks;

public class WeChatLogin : MonoBehaviour
{
    public string SERVER_LOGIN_URL = "http://43.128.29.250:80/wxlogin";
    public string WX_APP_ID = "wx735dd8b2e0fda8fe";

    public event Action<string> OnLoginSuccess;
    public event Action<string> OnLoginFailed;

    private bool _tryingToInitSDK = false;
    private bool _SDKInited = false;
    
    private void Start()
    {
        InitWXSDK();
    }

    public async void BeginLogin()
    {
        while (_tryingToInitSDK)
            await Task.Yield();
        if (!_SDKInited)
            InitWXSDK(StartLogin);
        else
            StartLogin();
    }

    private void InitWXSDK(Action onSuccess = null)
    {
        if (_tryingToInitSDK || _SDKInited)
            return;
        
        _tryingToInitSDK = true;
        WX.InitSDK((code) =>
        {
            _tryingToInitSDK = false;
            if (code != 0 && code != 200)
            {
                string error = "WeChat SDK init failed: " + code;
                Debug.LogError(error);
                OnLoginFailed?.Invoke(error);
            }
            else
            {
                _SDKInited = true;
                if (onSuccess != null) onSuccess();
            }
        });
    }

    private void StartLogin()
    {
        WX.Login(new LoginOption
        {
            success = (res) =>
            {
                Debug.Log("StartLogin res.code: " + res.code);
                StartCoroutine(SendCodeToServer(res.code));
            },
            fail = (err) =>
            {
                string error = "StartLogin err.errMsg: " + err.errMsg;
                Debug.LogError(error);
                OnLoginFailed?.Invoke(error);
            }
        });
    }

    private IEnumerator SendCodeToServer(string code)
    {
        WWWForm form = new WWWForm();
        form.AddField("code", code);
        form.AddField("appid", WX_APP_ID);

        using (UnityWebRequest request = UnityWebRequest.Post(SERVER_LOGIN_URL, form))
        {
            yield return request.SendWebRequest();

            if (request.result == UnityWebRequest.Result.Success)
            {
                string jsonResponse = request.downloadHandler.text;
                Debug.Log("SendCodeToServer jsonResponse: " + jsonResponse);

                try
                {
                    LoginResponse response = JsonUtility.FromJson<LoginResponse>(jsonResponse);

                    if (string.IsNullOrEmpty(response.errMsg))
                    {
                        Debug.Log("SendCodeToServer openId: " + response.openid);
                        OnLoginSuccess?.Invoke(response.openid);
                    }
                    else
                    {
                        Debug.LogError("SendCodeToServer response.errMsg: " + response.errMsg);
                        OnLoginFailed?.Invoke(response.errMsg);
                    }
                }
                catch (Exception e)
                {
                    string error = "SendCodeToServer e.Message: " + e.Message;
                    Debug.LogError(error);
                    OnLoginFailed?.Invoke(error);
                }
            }
            else
            {
                string error = "SendCodeToServer request.error: " + request.error;
                Debug.LogError(error);
                OnLoginFailed?.Invoke(error);
            }
        }
    }

    [System.Serializable]
    private class LoginResponse
    {
        public string openid;
        public string errMsg;
    }
}