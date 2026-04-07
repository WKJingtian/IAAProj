using System;
using System.Linq;
using System.Threading.Tasks;
using Unity.VisualScripting;

public class Player : MonoSingleton<Player>
{
    private PlayerDataService _playerDataService;
    private RoomDataService _roomDataService;
    private PlayerDataResponse _myData;
    private RoomDataResponse _myRoomData;
    public PlayerDataResponse MyData => _myData;
    public RoomDataResponse MyRoomData => _myRoomData;
    
    public async Task Initialize()
    {
        await WeChatLoginManager.Instance.BeginLoginAsync();
        _playerDataService = new(WeChatLoginManager.Instance);
        _roomDataService = new(WeChatLoginManager.Instance);
        await _playerDataService.FetchPlayerDataAsync(data => _myData = data);
        await _roomDataService.FetchRoomDataAsync(data => _myRoomData = data);

        MessageBus.Global.Listen<PlayerDataResponse>(MessageChannels.OnPlayerDataUpdateRequestComplete, this, OnPlayerInfoUpdated);
        MessageBus.Global.Listen<RoomDataResponse>(MessageChannels.OnRoomDataRequestComplete, this, OnRoomDataUpdated);
        MessageBus.Global.Listen<TriggerEventResponse>(MessageChannels.OnEventTriggerRequestComplete, this, OnEventTriggered);
        MessageBus.Global.Listen<UpgradeFurnitureResponse>(MessageChannels.OnFurnitureUpgradeRequestComplete, this, OnFurnitureUpgraded);
    }

    void OnPlayerInfoUpdated(PlayerDataResponse playerDataResponse)
    {
        _myData = playerDataResponse;
        MessageBus.Global.Dispatch(MessageChannels.OnPlayerDataUpdated);
    }

    void OnRoomDataUpdated(RoomDataResponse roomDataResponse)
    {
        _myRoomData = roomDataResponse;
        MessageBus.Global.Dispatch(MessageChannels.OnRoomDataUpdated);
    }

    void OnEventTriggered(TriggerEventResponse triggerEventResponse)
    {
        _myData.Cash = triggerEventResponse.Cash;
        _myData.Shield = triggerEventResponse.Shield;
        _myData.Energy = triggerEventResponse.Energy;
        _myData.EnergyRecoverAt = triggerEventResponse.EnergyRecoverAt;
        _myData.Asset =  triggerEventResponse.Asset;
        UInt16[] newHistory = _myData.EventHistory.Concat(triggerEventResponse.EventHistoryDelta ?? Array.Empty<UInt16>()).ToArray();
        _myData.EventHistory = newHistory;
        UInt64[] newTargets = _myData.EventTargetPlayerIDs.Concat(triggerEventResponse.TargetPlayerIDs ?? Array.Empty<UInt64>()).ToArray();
        _myData.EventTargetPlayerIDs = newTargets;
        MessageBus.Global.Dispatch(MessageChannels.OnPlayerDataUpdated);
    }

    void OnFurnitureUpgraded(UpgradeFurnitureResponse upgradeFurnitureResponse)
    {
        _myRoomData = new RoomDataResponse
        {
            CurrentRoomID = upgradeFurnitureResponse.CurrentRoomID,
            FurnitureLevels = upgradeFurnitureResponse.FurnitureLevels
        };

        if (_myData != null)
        {
            _myData.Cash = upgradeFurnitureResponse.Cash;
            _myData.Asset = upgradeFurnitureResponse.Asset;
        }

        MessageBus.Global.Dispatch(MessageChannels.OnRoomDataUpdated);
        MessageBus.Global.Dispatch(MessageChannels.OnPlayerDataUpdated);
    }
}
