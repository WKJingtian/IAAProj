# WeChatLoginManager Quick Notes

Updated: 2026-03-11 (latest)

## Login Flow

1. `MainPageUI.OnLoginBtnClicked` calls `WeChatLoginManager.BeginLogin()`.
2. `BeginLogin()` triggers `RequestWeChatPermissions()` first.
3. If `_isLoggingIn == true`, return immediately.
4. If `HasValidSession == true`, call `NotifySessionReady(CurrentSession)` and skip `WX.Login`.
5. Otherwise:
   - `_weChatAuthProvider.Login(...)`
   - `WX.InitSDK` (if not inited)
   - `WX.Login` gets a `code`
   - `GatewayAuthClient.LoginWithCode(code)` calls `/login`
   - `OnLoginSucceeded` -> `_sessionStore.SetSession(session)` -> `NotifySessionReady(session)`

## Token Cache Behavior

- `AuthSessionStore` loads `openid/token` from `PlayerPrefs` in constructor.
- If cache is valid, `BeginLogin` dispatches `AuthLoginSuccess/AuthSessionReady/AuthReady` directly.
- Cache clear points:
  - `Logout()`
  - Request error containing `"401"` in `HandleGameRequestError`

## Unified Permission Flow (official path)

1. `RequestWeChatPermissions`
2. `WX.GetPrivacySetting`:
   - if `privacyContractName` is empty -> fail fast and throw
3. `WX.RequirePrivacyAuthorize`:
   - uses official privacy popup (no custom privacy UI)
4. `WX.GetSetting`:
   - if `scope.userInfo` not granted -> `WX.OpenSetting`
5. `WX.GetUserInfo`
6. `CompleteUserInfoPermission`:
   - `_hasRequestedWeChatPermissions = true`
   - `MyName = nickName`
   - Dispatch `AuthUserInfoReady`
   - Call `IAA_RequestFriendsStateData()` in the same chain

## Important Events

- Login success: `AuthLoginSuccess`, `AuthSessionReady`, `AuthReady`
- Login failed: `AuthLoginFailed`
- Permission success: `AuthUserInfoReady`
- Permission failed: `AuthPermissionFailed`
- Request failed: `AuthRequestFailed`

## Strict Error Principle

- No silent fallback for malformed SDK payloads (`FailFast` + throw).
- Privacy contract missing is treated as a hard error:
  - `wx.getPrivacySetting` success with empty `privacyContractName` -> fail immediately.
