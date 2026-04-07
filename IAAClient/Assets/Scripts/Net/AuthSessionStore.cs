using UnityEngine;

public class AuthSessionStore
{
    private AuthSession _current = new AuthSession();

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
        get { return _current.IsValid; }
    }

    public void SetSession(AuthSession session)
    {
        _current = session;
        SaveToPrefs();
    }

    public void Clear()
    {
        _current = new AuthSession();
        PlayerPrefs.DeleteKey(Consts.AuthSessionOpenIdKey);
        PlayerPrefs.DeleteKey(Consts.AuthSessionTokenKey);
        PlayerPrefs.Save();
    }

    private void SaveToPrefs()
    {
        if (_current.IsValid)
        {
            PlayerPrefs.SetString(Consts.AuthSessionOpenIdKey, _current.openid);
            PlayerPrefs.SetString(Consts.AuthSessionTokenKey, _current.token);
            PlayerPrefs.Save();
            return;
        }

        PlayerPrefs.DeleteKey(Consts.AuthSessionOpenIdKey);
        PlayerPrefs.DeleteKey(Consts.AuthSessionTokenKey);
        PlayerPrefs.Save();
    }

    private void LoadFromPrefs()
    {
        string openid = PlayerPrefs.GetString(Consts.AuthSessionOpenIdKey, "");
        string token = PlayerPrefs.GetString(Consts.AuthSessionTokenKey, "");

        AuthSession loaded = new AuthSession
        {
            openid = openid,
            token = token
        };

        _current = loaded;
        if (!_current.IsValid)
        {
            PlayerPrefs.DeleteKey(Consts.AuthSessionOpenIdKey);
            PlayerPrefs.DeleteKey(Consts.AuthSessionTokenKey);
            PlayerPrefs.Save();
        }
    }
}
