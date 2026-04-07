using System;

public static class ErrorCode
{
    // Shared HTTP error codes. Keep these values aligned with:
    // IAAServer/common/errorcode/errorcode.go

    // No error.
    public const ushort OK = 0;

    // Unexpected server-side failure.
    public const ushort INTERNAL_ERROR = 1;
    // HTTP method is not allowed for the route.
    public const ushort INVALID_METHOD = 2;
    // Request shape is invalid but no more specific code exists.
    public const ushort INVALID_REQUEST = 3;
    // Caller is not authenticated or not allowed to access the route.
    public const ushort UNAUTHORIZED = 4;
    // Route does not exist.
    public const ushort ROUTE_NOT_FOUND = 5;
    // Gateway could not find a healthy upstream service.
    public const ushort UPSTREAM_UNAVAILABLE = 6;
    // Gateway failed while proxying to the upstream service.
    public const ushort UPSTREAM_PROXY_FAILED = 7;

    // Auth header or forwarded identity header is missing.
    public const ushort AUTH_MISSING_HEADER = 100;
    // Authorization header is not a Bearer token.
    public const ushort AUTH_INVALID_BEARER = 101;
    // Bearer token exists but is empty.
    public const ushort AUTH_EMPTY_BEARER_TOKEN = 102;
    // JWT verification failed or claims are invalid.
    public const ushort AUTH_INVALID_TOKEN = 103;
    // Token or forwarded request is missing openid identity.
    public const ushort AUTH_MISSING_OPENID = 104;

    // Login request is missing the WeChat code.
    public const ushort LOGIN_CODE_EMPTY = 120;
    // Client appid does not match server config.
    public const ushort LOGIN_APPID_MISMATCH = 121;
    // Server failed to call the WeChat login API.
    public const ushort LOGIN_WX_REQUEST_FAILED = 122;
    // WeChat login API response could not be parsed.
    public const ushort LOGIN_WX_RESPONSE_INVALID = 123;
    // WeChat login API returned an application-level error.
    public const ushort LOGIN_WX_API_ERROR = 124;
    // JWT generation failed after login succeeded.
    public const ushort LOGIN_JWT_GENERATION_FAILED = 125;

    // trigger_event request body is malformed.
    public const ushort TRIGGER_EVENT_PAYLOAD_INVALID = 200;
    // trigger_event multiplier is invalid.
    public const ushort TRIGGER_EVENT_MULTIPLIER_INVALID = 201;
    // Player lacks enough energy to trigger the event.
    public const ushort TRIGGER_EVENT_INSUFFICIENT_ENERGY = 202;

    // upgrade_furniture request body is malformed.
    public const ushort UPGRADE_FURNITURE_PAYLOAD_INVALID = 300;
    // furniture_id is missing.
    public const ushort UPGRADE_FURNITURE_ID_REQUIRED = 301;
    // furniture_id is negative or otherwise invalid.
    public const ushort UPGRADE_FURNITURE_ID_INVALID = 302;
    // furniture id does not exist.
    public const ushort UPGRADE_FURNITURE_NOT_FOUND = 303;
    // furniture is not in the player's current room.
    public const ushort UPGRADE_FURNITURE_NOT_IN_CURRENT_ROOM = 304;
    // furniture is already at max level.
    public const ushort UPGRADE_FURNITURE_MAX_LEVEL = 305;
    // player lacks enough cash to upgrade furniture.
    public const ushort UPGRADE_FURNITURE_INSUFFICIENT_CASH = 306;

    // Client-only local errors. These codes are never returned by the server.
    public const ushort CLIENT_NO_VALID_SESSION = 65000;
    public const ushort CLIENT_REQUEST_FAILED = 65001;
    public const ushort CLIENT_RESPONSE_PARSE_FAILED = 65002;
    public const ushort CLIENT_RESPONSE_INVALID = 65003;
    public const ushort CLIENT_LOGIN_RESPONSE_INVALID = 65004;
    public const ushort CLIENT_WECHAT_LOGIN_FAILED = 65010;
    public const ushort CLIENT_WECHAT_SDK_INIT_FAILED = 65011;
    public const ushort CLIENT_WECHAT_PRIVACY_SETTING_FAILED = 65012;
    public const ushort CLIENT_WECHAT_PRIVACY_AUTHORIZE_FAILED = 65013;
    public const ushort CLIENT_WECHAT_GET_SETTING_FAILED = 65014;
    public const ushort CLIENT_WECHAT_OPEN_SETTING_FAILED = 65015;
    public const ushort CLIENT_WECHAT_USER_NOT_AUTHORIZED = 65016;
    public const ushort CLIENT_WECHAT_GET_USER_INFO_FAILED = 65017;
    public const ushort CLIENT_UNHANDLED = 65535;
}

public sealed class AppErrorException : Exception
{
    public ushort Code { get; }

    public AppErrorException(ushort code) : base($"error_code={code}")
    {
        Code = code;
    }

    public AppErrorException(ushort code, Exception innerException) : base($"error_code={code}", innerException)
    {
        Code = code;
    }
}

public static class ErrorCodeUtil
{
    public static ushort ExtractCode(Exception ex, ushort fallbackCode = ErrorCode.CLIENT_UNHANDLED)
    {
        if (ex is AppErrorException appError)
        {
            return appError.Code;
        }

        return fallbackCode;
    }

    public static bool IsAuthFailure(ushort code)
    {
        return code == ErrorCode.UNAUTHORIZED ||
               code == ErrorCode.AUTH_MISSING_HEADER ||
               code == ErrorCode.AUTH_INVALID_BEARER ||
               code == ErrorCode.AUTH_EMPTY_BEARER_TOKEN ||
               code == ErrorCode.AUTH_INVALID_TOKEN ||
               code == ErrorCode.AUTH_MISSING_OPENID;
    }
}
