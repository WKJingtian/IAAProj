using TMPro;
using System;
using System.Collections;
using System.Collections.Generic;
using System.Text;
using UnityEngine;
using UnityEngine.Scripting;
using UnityEngine.UI;
using WeChatWASM;

[Preserve]
public class MainPageUI : PanelBase
{
    [SerializeField] Button _quitBtn;
    [SerializeField] Button _upgradeFurnitureBtn;
    [SerializeField] Button _triggerEventBtn;
    [SerializeField] TextMeshProUGUI _accountInfoField;
    [SerializeField] TextMeshProUGUI _roomInfoField;
    [SerializeField] TextMeshProUGUI _debugTextField;

    private WeChatLoginManager _loginManager;
    private DebugValService _debugValService;
    private RoomDataService _roomDataService;
    private TriggerEventService _eventService;
    private long _lastEnergyCountdownSecond = -1;
    
    protected override void Awake()
    {
        base.Awake();
        
        _loginManager = WeChatLoginManager.Instance;
        if (_loginManager == null)
        {
            throw new InvalidOperationException("MainPageUI.WeChatLoginManager.Instance is not assigned.");
        }

        _debugValService = new DebugValService(_loginManager);
        _roomDataService = new RoomDataService(_loginManager);
        _eventService = new TriggerEventService(_loginManager);

        _quitBtn.onClick.AddListener(OnQuitBtnClicked);
        
        _upgradeFurnitureBtn.onClick.AddListener(OnUpgradeFurnitureBtnClicked);
        _triggerEventBtn.onClick.AddListener(OnTriggerEventBtnClicked);

        RefreshPlayerData();
        RefreshRoomData();
        _debugTextField.text = "";
        MessageBus.Global.Listen(MessageChannels.OnPlayerDataUpdated, this, RefreshPlayerData);
        MessageBus.Global.Listen(MessageChannels.OnRoomDataUpdated, this, RefreshRoomData);
        
        MessageBus.Global.Listen(MessageChannels.UIDisplayDebugMessage, this, (string MSG) => _accountInfoField.text = MSG);
        MessageBus.Global.Listen(MessageChannels.AppErrorReceived, 
            this, (ushort errCode) => _debugTextField.text = LocalizationManager.GetLocalizedString($"error_{errCode}"));
    }

    private void OnDestroy()
    {
        MessageBus.Global.UnlistenAll(this);
    }

    int _debugIndex = 0;
    void OnUpgradeFurnitureBtnClicked()
    {
        var data = Player.Instance.MyRoomData;
        var roomConfig = GameConfigManager.GetRoomConfig(data.CurrentRoomID);
        if (!roomConfig.HasValue || roomConfig.Value.furnitures == null || roomConfig.Value.furnitures.Count == 0)
        {
            ShowDebugText($"Missing room config for room {data.CurrentRoomID}.");
            return;
        }

        var furnitureIDs = roomConfig.Value.furnitures;
        _roomDataService.UpgradeFurnitureAsync(
            furnitureIDs[(++_debugIndex) % furnitureIDs.Count],
            reply =>
        { });
    }

    void OnTriggerEventBtnClicked()
    {
        _eventService.TriggerEventAsync(reply =>
        {
            ClearDebugMsgQueue();
            bool firstMsg = true;
            foreach (var eventID in reply.EventHistoryDelta)
            {
                ShowDebugText(LocalizationManager.GetLocalizedString($"event_{eventID}_name"), firstMsg);
                ShowDebugText(LocalizationManager.GetLocalizedString($"event_{eventID}_desc"), false);
                firstMsg = false;
            }

            if (reply.TargetPlayerIDs != null && reply.TargetPlayerIDs.Length > 0)
            {
                ShowDebugText($"target ids: {string.Join(", ", reply.TargetPlayerIDs)}", false);
            }
        });
    }

    void OnQuitBtnClicked()
    {
        WX.ExitMiniProgram(new ExitMiniProgramOption());
    }

    #region Debug code
    void GetDebugVal()
    {
        _debugValService.FetchDebugValAsync(val => ShowDebugText($"Get Debug Value: {val}"));
    }

    void IncDebugVal()
    {
        _debugValService.IncrementDebugValAsync(val => ShowDebugText($"Increment Debug Value: {val}"));
    }

    struct DebugMsg
    {
        public string msg;
        public bool clear;
    }

    Queue<DebugMsg> _msgQueue = new Queue<DebugMsg>();
    float _msgCD = 0.0f;
    private void ShowDebugText(string text, bool clear = true)
    {
        string namePrefix = _loginManager.MyName;
        _msgQueue.Enqueue(new DebugMsg(){ msg = text, clear = clear});
        Debug.LogWarning(text);
    }

    private void ClearDebugMsgQueue()
    {
        _msgQueue.Clear();
    }
    void UpdateDebugMsgQueue(float dt)
    {
        if (_msgCD > 0)
            _msgCD -= dt;
        else if (_msgQueue.Count > 0)
        {
            _msgCD = 0.5f;
            var nextMsg =  _msgQueue.Dequeue();
            _debugTextField.text = nextMsg.clear ?
                nextMsg.msg :
                _debugTextField.text + '\n' +  nextMsg.msg;
        }
    }
    #endregion

    void Update()
    {
        UpdateDebugMsgQueue(Time.deltaTime);
        UpdateEnergyCountdown();
    }

    void RefreshPlayerData()
    {
        StringBuilder sb = new();
        var data = Player.Instance.MyData;
        sb.Append("ID:" + data.PlayerID.ToString());
        sb.AppendLine();
        sb.Append(LocalizationManager.GetLocalizedString("item_0_name") + ":" + data.Cash.ToString());
        sb.AppendLine();
        sb.Append(LocalizationManager.GetLocalizedString("item_1_name") + ":" + data.Asset.ToString());
        sb.AppendLine();
        sb.Append(LocalizationManager.GetLocalizedString("item_2_name") + ":" + data.Energy.ToString());
        if (TryGetEnergyRecoverText(data, out string energyRecoverText))
        {
            sb.Append(" (");
            sb.Append(LocalizationManager.GetLocalizedString("next_energy_in", energyRecoverText));
            sb.Append(")");
        }
        sb.AppendLine();
        sb.Append(LocalizationManager.GetLocalizedString("item_3_name") + ":" + data.Shield.ToString());

        _accountInfoField.text = sb.ToString();
    }

    void RefreshRoomData()
    {
        StringBuilder sb = new();
        var data = Player.Instance.MyRoomData;
        sb.Append(LocalizationManager.GetLocalizedString($"room_{data.CurrentRoomID}_name"));
        var roomConfig = GameConfigManager.GetRoomConfig(data.CurrentRoomID);
        if (!roomConfig.HasValue || roomConfig.Value.furnitures == null)
            return;

        int count = Mathf.Min(roomConfig.Value.furnitures.Count, data.FurnitureLevels?.Length ?? 0);
        for (int i = 0; i < count; i++)
        {
            sb.AppendLine();
            var furnitureConfig = GameConfigManager.GetFurnitureConfig(roomConfig.Value.furnitures[i]);
            if (!furnitureConfig.HasValue) continue;
            sb.Append(LocalizationManager.GetLocalizedString($"{furnitureConfig?.key}_name"));
            int star = 0;
            while (star < data.FurnitureLevels[i])
            {
                sb.Append(LocalizationManager.GetLocalizedString("star"));
                star++;
            }
        }
        
        _roomInfoField.text = sb.ToString();
    }

    private void UpdateEnergyCountdown()
    {
        var player = Player.Instance;
        if (player == null || player.MyData == null)
            return;

        long currentSecond = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        if (currentSecond == _lastEnergyCountdownSecond)
            return;

        _lastEnergyCountdownSecond = currentSecond;
        if (player.MyData.EnergyRecoverAt > 0)
            RefreshPlayerData();
    }

    private static bool TryGetEnergyRecoverText(PlayerDataResponse data, out string text)
    {
        text = null;
        if (data == null || data.EnergyRecoverAt == 0)
            return false;

        long now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        long remainingSeconds = (long)data.EnergyRecoverAt - now;
        if (remainingSeconds <= 0)
            return false;

        if (remainingSeconds > 3600)
        {
            long totalMinutes = (remainingSeconds + 59) / 60;
            long hours = totalMinutes / 60;
            long minutes = totalMinutes % 60;
            text = LocalizationManager.GetLocalizedString("time_hour_minute", hours, minutes);
            return true;
        }

        long minutesPart = remainingSeconds / 60;
        long secondsPart = remainingSeconds % 60;
        text = LocalizationManager.GetLocalizedString("time_min_second", minutesPart, secondsPart);
        return true;
    }
}
