using System.Collections;
using System.Collections.Generic;
using TMPro;
using UnityEngine;
using UnityEngine.UI;
using WeChatWASM;

public class MainPageUI : MonoBehaviour
{
    [SerializeField] WeChatLogin _loginComponent;
    [SerializeField] Button _loginBtn;
    [SerializeField] Button _quitBtn;
    [SerializeField] TextMeshProUGUI _debugTextField;

    private void Awake()
    {
        _loginBtn.onClick.AddListener(OnLoginBtnClicked);
        _quitBtn.onClick.AddListener(OnQuitBtnClicked);

        WX.OnShow((res) =>
        {
            _debugTextField.text = $"force call WX SDK";
        });
        
        _loginComponent.OnLoginSuccess += openid =>
        {
            _debugTextField.text = $"wechat login success, openid: {openid}";
        };
        _loginComponent.OnLoginFailed += error =>
        {
            _debugTextField.text = $"wechat login failed: {error}";
        };
    }

    void OnLoginBtnClicked()
    {
        _debugTextField.text = $"login button clicked (APP_ID -> {_loginComponent.WX_APP_ID})";
        _loginComponent.BeginLogin();
    }

    void OnQuitBtnClicked()
    {
        _debugTextField.text = $"quit button clicked";
        Application.Quit();
    }
    
    public void ShowDebugText(string text)
    {
        _debugTextField.text = text;
    }
}
