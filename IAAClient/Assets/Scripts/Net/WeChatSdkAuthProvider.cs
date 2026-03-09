using System;
using System.Collections.Generic;
using UnityEngine;
using WeChatWASM;

public class WeChatSdkAuthProvider
{
    private bool _tryingToInitSDK;
    private bool _sdkInited;
    private readonly List<Action> _pendingOnReady = new List<Action>();
    private readonly List<Action<string>> _pendingOnError = new List<Action<string>>();

    public void Login(Action<string> onCodeReceived, Action<string> onFailed)
    {
        EnsureInited(
            () =>
            {
                WX.Login(new LoginOption
                {
                    success = (res) =>
                    {
                        if (string.IsNullOrEmpty(res.code))
                        {
                            onFailed?.Invoke("WeChat login failed: empty code");
                            return;
                        }
                        onCodeReceived?.Invoke(res.code);
                    },
                    fail = (err) =>
                    {
                        onFailed?.Invoke("WeChat login failed: " + err.errMsg);
                    }
                });
            },
            onFailed
        );
    }

    private void EnsureInited(Action onReady, Action<string> onFailed)
    {
        if (_sdkInited)
        {
            onReady?.Invoke();
            return;
        }

        _pendingOnReady.Add(onReady);
        _pendingOnError.Add(onFailed);

        if (_tryingToInitSDK)
        {
            return;
        }

        _tryingToInitSDK = true;
        WX.InitSDK((code) =>
        {
            _tryingToInitSDK = false;

            if (code != 0 && code != 200)
            {
                string error = "WeChat SDK init failed: " + code;
                Debug.LogError(error);
                for (int i = 0; i < _pendingOnError.Count; i++)
                {
                    _pendingOnError[i]?.Invoke(error);
                }
                _pendingOnReady.Clear();
                _pendingOnError.Clear();
                return;
            }

            _sdkInited = true;
            for (int i = 0; i < _pendingOnReady.Count; i++)
            {
                _pendingOnReady[i]?.Invoke();
            }
            _pendingOnReady.Clear();
            _pendingOnError.Clear();
        });
    }
}
