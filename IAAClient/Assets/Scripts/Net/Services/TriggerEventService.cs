using System;
using System.Threading.Tasks;

public class TriggerEventService : ServiceBase
{
    public TriggerEventService(WeChatLoginManager loginManager) : base(loginManager)
    {
    }

    public async Task TriggerEventAsync(Action<TriggerEventResponse> cb)
    {
        await TriggerEventAsync(1, cb);
    }

    public async Task TriggerEventAsync(int multiplier, Action<TriggerEventResponse> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;
        if (multiplier <= 0)
        {
            HandleRequestError(ErrorCode.TRIGGER_EVENT_MULTIPLIER_INVALID);
            return;
        }

        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.TriggerEvent, token,
                new TriggerEventRequest
                {
                    Multiplier = multiplier
                });
            cb?.Invoke(response);
            MessageBus.Global.Dispatch(MessageChannels.OnEventTriggerRequestComplete, response);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }
}
