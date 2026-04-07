using System;
using System.Threading.Tasks;

public class PlayerDataService : ServiceBase
{
    public PlayerDataService(WeChatLoginManager loginManager) : base(loginManager)
    {
    }

    public async Task FetchPlayerDataAsync(Action<PlayerDataResponse> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;

        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.GetOrSetPlayerData, token);
            cb?.Invoke(response);
            MessageBus.Global.Dispatch(MessageChannels.OnPlayerDataUpdateRequestComplete, response);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }
}
