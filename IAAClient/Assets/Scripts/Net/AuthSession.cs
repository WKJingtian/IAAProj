using System;

[Serializable]
public class AuthSession
{
    public string openid = string.Empty;
    public string token = string.Empty;

    public bool IsValid
    {
        get { return openid.Length > 0 && token.Length > 0; }
    }
}
