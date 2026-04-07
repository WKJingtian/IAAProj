using System;
using System.Threading.Tasks;

public class ServiceBase
{
    protected readonly WeChatLoginManager _loginManager;
    protected readonly GameApiClient _gameApiClient;

    public ServiceBase(WeChatLoginManager loginManager)
    {
        if (loginManager == null)
            throw new ArgumentNullException(nameof(loginManager));

        _loginManager = loginManager;
        _gameApiClient = new GameApiClient(GlobalConfigs.GatewayBaseUrl);
    }

    protected bool TryGetCurrentToken(out string token)
    {
        if (_loginManager.HasValidSession)
        {
            token = _loginManager.CurrentSession.token;
            return true;
        }

        token = null;
        BroadcastErrorCode(ErrorCode.CLIENT_NO_VALID_SESSION, true);
        return false;
    }

    protected void HandleRequestError(Exception ex)
    {
        ushort code = ErrorCodeUtil.ExtractCode(ex);
        BroadcastErrorCode(code, true);
    }

    protected void HandleRequestError(ushort code)
    {
        BroadcastErrorCode(code, true);
    }

    protected void BroadcastErrorCode(ushort code, bool dispatchAuthRequestFailed)
    {
        if (ErrorCodeUtil.IsAuthFailure(code))
        {
            _loginManager.ClearSession();
        }

        MessageBus.Global.Dispatch(MessageChannels.AppErrorReceived, code);
        if (dispatchAuthRequestFailed)
        {
            MessageBus.Global.Dispatch(MessageChannels.AuthRequestFailed, code);
        }
    }
}
