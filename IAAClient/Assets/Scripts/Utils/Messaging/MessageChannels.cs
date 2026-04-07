public static class MessageChannels
{
    public const string AppErrorReceived = "App/error_received";
    public const string AuthLoginSuccess = "Auth/login_success";
    public const string AuthLoginFailed = "Auth/login_failed";
    public const string AuthSessionReady = "Auth/session_ready";
    public const string AuthReady = "Auth/ready";
    public const string AuthUserInfoReady = "Auth/user_info_ready";
    public const string AuthPermissionFailed = "Auth/permission_failed";
    public const string AuthRequestFailed = "Auth/request_failed";

    public const string UIDisplayDebugMessage = "UI/display_debug_message";

    public const string OnPlayerDataUpdateRequestComplete = "NetEvent/player_data_update_request_complete";
    public const string OnRoomDataRequestComplete = "NetEvent/room_data_request_complete";
    public const string OnEventTriggerRequestComplete = "NetEvent/event_trigger_request_complete";
    public const string OnFurnitureUpgradeRequestComplete = "NetEvent/furniture_upgrade_request_complete";

    public const string GameTickMessage = "GameEvent/tick";
    public const string OnPlayerDataUpdated = "GameEvent/player_data_updated";
    public const string OnRoomDataUpdated = "GameEvent/room_data_updated";
}
