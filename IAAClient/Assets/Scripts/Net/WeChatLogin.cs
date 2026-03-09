using System;
using UnityEngine;

public class WeChatLogin : MonoBehaviour
{
    [Header("Gateway")]
    public string GATEWAY_BASE_URL = "http://43.128.29.250:80";

    [Header("WeChat")]
    public string WX_APP_ID = "wx735dd8b2e0fda8fe";

    // Backward-compatible: old listener can still consume openid.
    public event Action<string> OnLoginSuccess;
    public event Action<string> OnLoginFailed;

    // New events for decoupled architecture.
    public event Action<AuthSession> OnSessionReady;
    public event Action<string> OnRequestFailed;

    private readonly AuthSessionStore _sessionStore = new AuthSessionStore();
    private WeChatSdkAuthProvider _weChatAuthProvider;
    private GatewayAuthClient _gatewayAuthClient;
    private GameApiClient _gameApiClient;
    private bool _isLoggingIn;

    public bool HasValidSession
    {
        get { return _sessionStore.HasValidSession; }
    }

    public AuthSession CurrentSession
    {
        get { return _sessionStore.Current; }
    }

    private void Awake()
    {
        EnsureClientsReady();
    }

    public void BeginLogin()
    {
        if (_isLoggingIn)
        {
            return;
        }
        if (_sessionStore.HasValidSession)
        {
            AuthSession session = _sessionStore.Current;
            OnLoginSuccess?.Invoke(session.openid);
            OnSessionReady?.Invoke(session);
            return;
        }

        EnsureClientsReady();
        _isLoggingIn = true;

        _weChatAuthProvider.Login(
            (code) =>
            {
                StartCoroutine(_gatewayAuthClient.LoginWithCode(
                    code,
                    (session) =>
                    {
                        _isLoggingIn = false;
                        _sessionStore.SetSession(session);
                        OnLoginSuccess?.Invoke(session.openid);
                        OnSessionReady?.Invoke(session);
                    },
                    (error) =>
                    {
                        _isLoggingIn = false;
                        OnLoginFailed?.Invoke(error);
                    }
                ));
            },
            (error) =>
            {
                _isLoggingIn = false;
                OnLoginFailed?.Invoke(error);
            }
        );
    }

    public void FetchDebugVal(Action<int> cb = null)
    {
        if (!EnsureSessionOrFail())
        {
            return;
        }

        EnsureClientsReady();
        StartCoroutine(_gameApiClient.GetDebugVal(
            _sessionStore.Current.token,
            (response) =>
            {
                if (cb != null) cb(response.debug_val);
            },
            HandleGameRequestError
        ));
    }

    public void IncrementDebugVal(Action<int> cb = null)
    {
        if (!EnsureSessionOrFail())
        {
            return;
        }

        EnsureClientsReady();
        StartCoroutine(_gameApiClient.IncrementDebugVal(
            _sessionStore.Current.token,
            (response) =>
            {
                if (cb != null) cb(response.debug_val);
            },
            HandleGameRequestError
        ));
    }

    public void Logout()
    {
        _sessionStore.Clear();
    }

    private void EnsureClientsReady()
    {
        if (_weChatAuthProvider == null)
        {
            _weChatAuthProvider = new WeChatSdkAuthProvider();
        }

        _gatewayAuthClient = new GatewayAuthClient(GATEWAY_BASE_URL, WX_APP_ID);
        _gameApiClient = new GameApiClient(GATEWAY_BASE_URL);
    }

    private bool EnsureSessionOrFail()
    {
        if (_sessionStore.HasValidSession)
        {
            return true;
        }

        OnRequestFailed?.Invoke("No valid session. Please call BeginLogin() first.");
        return false;
    }

    private void HandleGameRequestError(string error)
    {
        if (!string.IsNullOrEmpty(error) && error.Contains("401"))
        {
            _sessionStore.Clear();
        }
        OnRequestFailed?.Invoke(error);
    }
}
