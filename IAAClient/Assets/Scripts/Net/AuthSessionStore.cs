using UnityEngine;

public class AuthSessionStore
{
    private const string OpenIdKey = "auth_session.openid";
    private const string TokenKey = "auth_session.token";

    private AuthSession _current;

    public AuthSessionStore()
    {
        LoadFromPrefs();
    }

    public AuthSession Current
    {
        get { return _current; }
    }

    public bool HasValidSession
    {
        get { return _current != null && _current.IsValid; }
    }

    public void SetSession(AuthSession session)
    {
        _current = session;
        SaveToPrefs();
    }

    public void Clear()
    {
        _current = null;
        PlayerPrefs.DeleteKey(OpenIdKey);
        PlayerPrefs.DeleteKey(TokenKey);
        PlayerPrefs.Save();
    }

    private void SaveToPrefs()
    {
        if (_current != null && _current.IsValid)
        {
            PlayerPrefs.SetString(OpenIdKey, _current.openid);
            PlayerPrefs.SetString(TokenKey, _current.token);
            PlayerPrefs.Save();
            return;
        }

        PlayerPrefs.DeleteKey(OpenIdKey);
        PlayerPrefs.DeleteKey(TokenKey);
        PlayerPrefs.Save();
    }

    private void LoadFromPrefs()
    {
        string openid = PlayerPrefs.GetString(OpenIdKey, "");
        string token = PlayerPrefs.GetString(TokenKey, "");

        AuthSession loaded = new AuthSession
        {
            openid = openid,
            token = token
        };

        if (loaded.IsValid)
        {
            _current = loaded;
            return;
        }

        _current = null;
        PlayerPrefs.DeleteKey(OpenIdKey);
        PlayerPrefs.DeleteKey(TokenKey);
        PlayerPrefs.Save();
    }
}
