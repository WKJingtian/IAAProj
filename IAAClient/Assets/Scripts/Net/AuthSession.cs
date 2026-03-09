using System;

[Serializable]
public class AuthSession
{
    public string openid;
    public string token;

    public bool IsValid
    {
        get { return !string.IsNullOrEmpty(openid) && !string.IsNullOrEmpty(token); }
    }
}
