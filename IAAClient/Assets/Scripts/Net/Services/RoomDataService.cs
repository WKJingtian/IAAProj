using System;
using System.Threading.Tasks;

public class RoomDataService : ServiceBase
{
    public RoomDataService(WeChatLoginManager loginManager) : base(loginManager)
    {
    }

    public async Task FetchRoomDataAsync(Action<RoomDataResponse> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;

        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.GetRoomData, token);
            cb?.Invoke(response);
            MessageBus.Global.Dispatch(MessageChannels.OnRoomDataRequestComplete, response);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }

    public async Task UpgradeFurnitureAsync(int furnitureID, Action<UpgradeFurnitureResponse> cb)
    {
        if (!TryGetCurrentToken(out string token)) return;
        if (furnitureID < 0)
        {
            HandleRequestError(ErrorCode.UPGRADE_FURNITURE_ID_INVALID);
            return;
        }

        try
        {
            var response = await _gameApiClient.SendAuthorizedRequestAsync(
                NetAPIs.UpgradeFurniture,
                token,
                new UpgradeFurnitureRequest
                {
                    FurnitureID = furnitureID
                });
            cb?.Invoke(response);
            MessageBus.Global.Dispatch(MessageChannels.OnFurnitureUpgradeRequestComplete, response);
        }
        catch (Exception ex)
        {
            HandleRequestError(ex);
        }
    }
}
