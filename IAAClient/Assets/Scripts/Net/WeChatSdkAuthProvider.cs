using System;
using System.Threading.Tasks;
using UnityEngine;
using WeChatWASM;

public class WeChatSdkAuthProvider
{
    private bool _sdkInited;
    private bool _hasUserProfile;
    private string _cachedNickName;

    // WX SDK initialization and WX login, returns an auth code from WX server
    public async Task<string> RequestLoginCodeAsync()
    {
        await EnsureInitializedAsync();

        TaskCompletionSource<string> tcs = new TaskCompletionSource<string>();
        WX.Login(new LoginOption
        {
            success = (res) => tcs.TrySetResult(res.code),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_LOGIN_FAILED))
        });

        return await tcs.Task;
    }

    // require privacy authorization
    public async Task<string> EnsureUserProfileReadyAsync()
    {
        if (_hasUserProfile)
        {
            return _cachedNickName;
        }

        await EnsureInitializedAsync();
        await GetPrivacySettingAsync();
        await RequirePrivacyAuthorizeAsync();
        await EnsureUserInfoScopeAsync();
        string nickName = await GetUserInfoAsync();
        _hasUserProfile = true;
        _cachedNickName = nickName;
        return nickName;
    }

    private async Task EnsureInitializedAsync()
    {
        if (_sdkInited)
        {
            return;
        }

        TaskCompletionSource<bool> tcs = new TaskCompletionSource<bool>();
        WX.InitSDK((code) =>
        {
            if (code != 0 && code != 200)
            {
                string error = "WeChat SDK init failed: " + code;
                Debug.LogError(error);
                tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_SDK_INIT_FAILED));
                return;
            }

            tcs.TrySetResult(true);
        });

        await tcs.Task;
        _sdkInited = true;
    }

    private static Task GetPrivacySettingAsync()
    {
        TaskCompletionSource<bool> tcs = new TaskCompletionSource<bool>();
        WX.GetPrivacySetting(new GetPrivacySettingOption
        {
            success = (res) => tcs.TrySetResult(true),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_PRIVACY_SETTING_FAILED))
        });
        return tcs.Task;
    }

    private static Task RequirePrivacyAuthorizeAsync()
    {
        TaskCompletionSource<bool> tcs = new TaskCompletionSource<bool>();
        WX.RequirePrivacyAuthorize(new RequirePrivacyAuthorizeOption
        {
            success = (_) => tcs.TrySetResult(true),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_PRIVACY_AUTHORIZE_FAILED))
        });
        return tcs.Task;
    }

    private static async Task EnsureUserInfoScopeAsync()
    {
        AuthSetting authSetting = await GetSettingAsync();
        if (IsUserInfoAuthorized(authSetting))
        {
            return;
        }

        authSetting = await OpenSettingAsync();
        if (!IsUserInfoAuthorized(authSetting))
        {
            throw new AppErrorException(ErrorCode.CLIENT_WECHAT_USER_NOT_AUTHORIZED);
        }
    }

    private static Task<AuthSetting> GetSettingAsync()
    {
        TaskCompletionSource<AuthSetting> tcs = new TaskCompletionSource<AuthSetting>();
        WX.GetSetting(new GetSettingOption
        {
            success = (res) => tcs.TrySetResult(res.authSetting),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_GET_SETTING_FAILED))
        });
        return tcs.Task;
    }

    private static Task<AuthSetting> OpenSettingAsync()
    {
        TaskCompletionSource<AuthSetting> tcs = new TaskCompletionSource<AuthSetting>();
        WX.OpenSetting(new OpenSettingOption
        {
            success = (res) => tcs.TrySetResult(res.authSetting),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_OPEN_SETTING_FAILED))
        });
        return tcs.Task;
    }

    private static Task<string> GetUserInfoAsync()
    {
        TaskCompletionSource<string> tcs = new TaskCompletionSource<string>();
        WX.GetUserInfo(new GetUserInfoOption
        {
            withCredentials = true,
            lang = GlobalConfigs.WeChatUserInfoLanguage,
            success = (res) => tcs.TrySetResult(res.userInfo.nickName),
            fail = (err) => tcs.TrySetException(new AppErrorException(ErrorCode.CLIENT_WECHAT_GET_USER_INFO_FAILED))
        });
        return tcs.Task;
    }

    private static bool IsUserInfoAuthorized(AuthSetting authSetting)
    {
        return authSetting.TryGetValue(Consts.ScopeUserInfo, out bool authorized) && authorized;
    }
}
