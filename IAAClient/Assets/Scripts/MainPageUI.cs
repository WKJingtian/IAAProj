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
    [SerializeField] Button _debugBtn1;
    [SerializeField] Button _debugBtn2;
    [SerializeField] TextMeshProUGUI _debugTextField;

    private void Awake()
    {
        _loginBtn.onClick.AddListener(OnLoginBtnClicked);
        _quitBtn.onClick.AddListener(OnQuitBtnClicked);
        
        _debugBtn1.onClick.AddListener(GetDebugVal);
        _debugBtn2.onClick.AddListener(IncDebugVal);

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

    void GetDebugVal()
    {
        _loginComponent.FetchDebugVal(val => _debugTextField.text = $"GetDebugVal {val}");
    }

    void IncDebugVal()
    {
        _loginComponent.IncrementDebugVal(val => _debugTextField.text = $"IncDebugVal {val}");
    }
    
    public void ShowDebugText(string text)
    {
        _debugTextField.text = text;
    }
}
