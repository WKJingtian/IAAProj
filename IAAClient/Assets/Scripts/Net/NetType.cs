using System;
using Newtonsoft.Json;

[UnityEngine.Scripting.Preserve]
[Serializable]
public class NetResponseBase
{
    [JsonProperty("err_msg",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt16 ErrMsg;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class DebugValResponse : NetResponseBase
{
    [JsonProperty("player_id",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64 PlayerID;

    [JsonProperty("openid",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public string OpenID;

    [JsonProperty("debug_val",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public int DebugVal;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class PlayerDataResponse : NetResponseBase
{
    [JsonProperty("player_id",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64 PlayerID;

    [JsonProperty("openid",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public string OpenID;

    [JsonProperty("creation_time",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64 CreationTime;

    [JsonProperty("cash",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Cash;

    [JsonProperty("asset",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Asset;

    [JsonProperty("energy",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Energy;

    [JsonProperty("energy_recover_at",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64 EnergyRecoverAt;

    [JsonProperty("shield",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Shield;

    [JsonProperty("event_history",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt16[] EventHistory;

    [JsonProperty("event_target_player_ids",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64[] EventTargetPlayerIDs;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class TriggerEventRequest
{
    [JsonProperty("multiplier",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Multiplier;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class TriggerEventResponse : NetResponseBase
{
    [JsonProperty("cash",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Cash;

    [JsonProperty("asset",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Asset;

    [JsonProperty("energy",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Energy;

    [JsonProperty("energy_recover_at",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64 EnergyRecoverAt;

    [JsonProperty("shield",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Shield;

    [JsonProperty("event_history_delta",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt16[] EventHistoryDelta;

    [JsonProperty("target_player_ids",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public UInt64[] TargetPlayerIDs;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class RoomDataResponse : NetResponseBase
{
    [JsonProperty("current_room_id",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 CurrentRoomID;

    [JsonProperty("furniture_levels",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32[] FurnitureLevels;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class UpgradeFurnitureRequest
{
    [JsonProperty("furniture_id",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 FurnitureID;
}

[UnityEngine.Scripting.Preserve]
[Serializable]
public class UpgradeFurnitureResponse : NetResponseBase
{
    [JsonProperty("current_room_id",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 CurrentRoomID;

    [JsonProperty("furniture_levels",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32[] FurnitureLevels;

    [JsonProperty("cash",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Cash;

    [JsonProperty("asset",
        Required = Required.Default,
        NullValueHandling = NullValueHandling.Ignore)]
    public Int32 Asset;
}
