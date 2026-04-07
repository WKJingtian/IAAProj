using System;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using UnityEngine;

public class WeChatLoginManager : MonoSingleton<WeChatLoginManager>
{
    private AuthSessionStore _sessionStore;
    private WeChatSdkAuthProvider _weChatAuthProvider;
    private GatewayAuthClient _gatewayAuthClient;
    private bool _isLoggingIn;
    private bool _hasPublishedUserInfo;

    public bool HasValidSession => _sessionStore.HasValidSession;
    public AuthSession CurrentSession => _sessionStore.Current;
    public string MyName { get; private set; } = string.Empty;

    protected override void Awake()
    {
        base.Awake();

        _sessionStore = new AuthSessionStore();
        _weChatAuthProvider = new WeChatSdkAuthProvider();
        _gatewayAuthClient = new GatewayAuthClient(GlobalConfigs.GatewayBaseUrl, GlobalConfigs.WeChatAppId);
    }

    public async Task BeginLoginAsync()
    {
        if (_isLoggingIn)
        {
            return;
        }

        _isLoggingIn = true;
        try
        {
            AuthSession session = null;
            if (_sessionStore.HasValidSession)
                // if cached data detected
                session = _sessionStore.Current;
            else
            {
                // login actually starts here
                var codeStr = await _weChatAuthProvider.RequestLoginCodeAsync();
                session = await _gatewayAuthClient.LoginWithCodeAsync(codeStr);
            }
            _sessionStore.SetSession(session);
            MessageBus.Global.Dispatch(MessageChannels.AuthLoginSuccess, session.openid);
            MessageBus.Global.Dispatch(MessageChannels.AuthSessionReady, session);
            MessageBus.Global.Dispatch(MessageChannels.AuthReady, session);

            // now we want users to allow us to use their nickname/profile photo/friend list
            string nickName = await _weChatAuthProvider.EnsureUserProfileReadyAsync();
            PublishUserInfo(nickName);
            _isLoggingIn = false;
        }
        catch (Exception ex)
        {
            _isLoggingIn = false;
            ushort code = ErrorCodeUtil.ExtractCode(ex);
            MessageBus.Global.Dispatch(MessageChannels.AppErrorReceived, code);
            MessageBus.Global.Dispatch(MessageChannels.AuthLoginFailed, code);
        }
    }

    [DllImport("__Internal")]
    private static extern void IAA_RequestFriendsStateData();

    public void ClearSession()
    {
        _sessionStore.Clear();
    }

    private void PublishUserInfo(string nickName)
    {
        bool shouldPublish = !_hasPublishedUserInfo || !string.Equals(MyName, nickName, StringComparison.Ordinal);
        MyName = nickName;
        if (!shouldPublish)
        {
            return;
        }

        _hasPublishedUserInfo = true;
        MessageBus.Global.Dispatch(MessageChannels.AuthUserInfoReady, MyName);
        IAA_RequestFriendsStateData();
    }
}
