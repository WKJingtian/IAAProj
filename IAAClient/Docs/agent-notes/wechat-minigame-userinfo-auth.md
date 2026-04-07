# WeChat Mini Game UserInfo Authorization Playbook

Last verified: 2026-03-10

## Official recommended flow

1. Call `wx.getSetting` and check `authSetting["scope.userInfo"]`.
2. If authorized: call `wx.getUserInfo` directly.
3. If not authorized: create `wx.createUserInfoButton` and wait for user tap.
4. If user denies: in a user-triggered path, call `wx.openSetting` to guide re-authorization.

Important:
- `wx.authorize({ scope: "scope.userInfo" })` does **not** show auth popup in mini game.
- `createUserInfoButton` requires privacy compliance config to be set.

## Unity WeChatWASM SDK mapping used in this project

- `WX.GetSetting(new GetSettingOption { success, fail, complete })`
  - `success` -> `GetSettingSuccessCallbackResult`
  - auth map is `res.authSetting` (`AuthSetting` extends `Dictionary<string, bool>`)
- `WX.GetUserInfo(new GetUserInfoOption { withCredentials, lang, success, fail })`
- `WX.CreateUserInfoButton(x, y, width, height, lang, withCredentials)` -> `WXUserInfoButton`
  - methods: `OnTap`, `OffTap`, `Show`, `Hide`, `Destroy`
- `WX.OpenSetting(new OpenSettingOption { success, fail, complete })`

## Implementation checklist

- Keep a single user-info button instance and always `Destroy()` it on success / destroy lifecycle.
- Use `"scope.userInfo"` constant for auth check.
- Guard concurrent requests with flags (checking/reading/open-setting).
- Only mark permission-complete after actually receiving user info.
- Send UI/debug messages for each stage so QA can trace flow quickly.

## YooAsset integration notes (for auth popup prefab)

- `AuthPopup` is loaded through YooAsset by address `AuthPopup` in package `DefaultPackage`.
- Collector config must include `Assets/Prefabs/UI/AuthPopup.prefab` with `AddressByFileName` rule.
- Runtime requires package initialize + version request + manifest update before any `LoadAssetAsync`.
- Build output must be copied to `StreamingAssets/yoo/DefaultPackage`.
- If runtime reports package/version/manifest errors:
  - run `Tools/YooAsset/Setup/Reset DefaultPackage Collector`
  - run `Tools/YooAsset/Build/Build DefaultPackage (ActiveTarget)`
  - rebuild mini game package and verify `StreamingAssets/yoo/DefaultPackage` exists

## References

- https://developers.weixin.qq.com/minigame/dev/guide/open-ability/user-info.html
- https://developers.weixin.qq.com/minigame/dev/api/open-api/authorize/wx.authorize.html
- https://developers.weixin.qq.com/minigame/dev/api/open-api/user-info/wx.createUserInfoButton.html
- https://developers.weixin.qq.com/minigame/dev/api/open-api/user-info/wx.getUserInfo.html
- https://developers.weixin.qq.com/minigame/dev/api/open-api/setting/wx.getSetting.html
- https://developers.weixin.qq.com/minigame/dev/api/open-api/setting/wx.openSetting.html
