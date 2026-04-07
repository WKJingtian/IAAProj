using System;
using System.Threading.Tasks;

public class DebugValService : ServiceBase
{
    public DebugValService(WeChatLoginManager loginManager) : base(loginManager)
    {
    }
    
    public async Task FetchDebugValAsync(Action<int> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;

        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.GetDebugVal, token);
            cb?.Invoke(response.DebugVal);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }

    public async Task IncrementDebugValAsync(Action<int> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;
        
        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.IncDebugVal, token);
            cb?.Invoke(response.DebugVal);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }
}
