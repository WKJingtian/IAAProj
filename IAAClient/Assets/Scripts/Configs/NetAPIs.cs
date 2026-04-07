using System;

public class NetAPI
{
    public readonly bool isGetMethod;
    public readonly string requestPath;
    public readonly Type returnType;

    protected NetAPI(bool isGet, string path, Type typ)
    {
        isGetMethod = isGet;
        requestPath = path;
        returnType = typ;
    }
    
    public string Method => isGetMethod ? "GET" : "POST";
}

public class NetAPI<TResponse> : NetAPI
{
    public NetAPI(bool isGet, string path) : base(isGet, path, typeof(TResponse))
    {
    }
}

public static class NetAPIs
{
    // debug
    public static readonly NetAPI<DebugValResponse> GetDebugVal =
        new NetAPI<DebugValResponse>(true, "/debug_val");

    public static readonly NetAPI<DebugValResponse> IncDebugVal =
        new NetAPI<DebugValResponse>(false, "/debug_val_inc");
    
    // player data
    public static readonly NetAPI<PlayerDataResponse> GetOrSetPlayerData =
        new NetAPI<PlayerDataResponse>(false, "/player_data");

    public static readonly NetAPI<RoomDataResponse> GetRoomData =
        new NetAPI<RoomDataResponse>(false, "/room_data");

    public static readonly NetAPI<TriggerEventResponse> TriggerEvent =
        new NetAPI<TriggerEventResponse>(false, "/trigger_event");

    public static readonly NetAPI<UpgradeFurnitureResponse> UpgradeFurniture =
        new NetAPI<UpgradeFurnitureResponse>(false, "/upgrade_furniture");
}
