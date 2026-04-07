package errorcode

// Code is the shared HTTP error code type used by server responses.
//
// Keep this list aligned with the Unity client file:
// IAAClient/Assets/Scripts/Net/Error.cs
type Code uint16

const (
	// OK means the request succeeded and no error is present.
	OK Code = 0

	// InternalError is used for unexpected server-side failures.
	InternalError Code = 1
	// InvalidMethod means the HTTP method is not allowed for the route.
	InvalidMethod Code = 2
	// InvalidRequest means the request shape is invalid but no more specific code exists.
	InvalidRequest Code = 3
	// Unauthorized means the caller is not authenticated or not allowed to access the route.
	Unauthorized Code = 4
	// RouteNotFound means the requested route does not exist.
	RouteNotFound Code = 5
	// UpstreamUnavailable means gateway could not find a healthy upstream service.
	UpstreamUnavailable Code = 6
	// UpstreamProxyFailed means gateway failed while proxying to the upstream service.
	UpstreamProxyFailed Code = 7

	// AuthMissingHeader means the auth header or forwarded identity header is missing.
	AuthMissingHeader Code = 100
	// AuthInvalidBearer means the Authorization header is not a Bearer token.
	AuthInvalidBearer Code = 101
	// AuthEmptyBearerToken means the Bearer token exists but is empty.
	AuthEmptyBearerToken Code = 102
	// AuthInvalidToken means JWT verification failed or claims are invalid.
	AuthInvalidToken Code = 103
	// AuthMissingOpenID means the token or forwarded request is missing openid identity.
	AuthMissingOpenID Code = 104

	// LoginCodeEmpty means the login request is missing the WeChat code.
	LoginCodeEmpty Code = 120
	// LoginAppIDMismatch means the client appid does not match server config.
	LoginAppIDMismatch Code = 121
	// LoginWXRequestFailed means the server failed to call the WeChat login API.
	LoginWXRequestFailed Code = 122
	// LoginWXResponseInvalid means the WeChat login API response could not be parsed.
	LoginWXResponseInvalid Code = 123
	// LoginWXAPIError means the WeChat login API returned an application-level error.
	LoginWXAPIError Code = 124
	// LoginJWTGenerationFailed means JWT generation failed after login succeeded.
	LoginJWTGenerationFailed Code = 125

	// TriggerEventPayloadInvalid means the trigger_event request body is malformed.
	TriggerEventPayloadInvalid Code = 200
	// TriggerEventMultiplierInvalid means multiplier is invalid.
	TriggerEventMultiplierInvalid Code = 201
	// TriggerEventInsufficientEnergy means the player lacks enough energy to trigger the event.
	TriggerEventInsufficientEnergy Code = 202

	// UpgradeFurniturePayloadInvalid means the upgrade_furniture request body is malformed.
	UpgradeFurniturePayloadInvalid Code = 300
	// UpgradeFurnitureFurnitureIDRequired means furniture_id is missing.
	UpgradeFurnitureFurnitureIDRequired Code = 301
	// UpgradeFurnitureFurnitureIDInvalid means furniture_id is negative or otherwise invalid.
	UpgradeFurnitureFurnitureIDInvalid Code = 302
	// UpgradeFurnitureFurnitureNotFound means the furniture id does not exist.
	UpgradeFurnitureFurnitureNotFound Code = 303
	// UpgradeFurnitureFurnitureNotInCurrentRoom means the furniture is not in the player's current room.
	UpgradeFurnitureFurnitureNotInCurrentRoom Code = 304
	// UpgradeFurnitureFurnitureMaxLevel means the furniture is already at max level.
	UpgradeFurnitureFurnitureMaxLevel Code = 305
	// UpgradeFurnitureInsufficientCash means the player lacks enough cash to upgrade furniture.
	UpgradeFurnitureInsufficientCash Code = 306
)
