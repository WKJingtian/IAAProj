using System;
using TMPro;
using UnityEngine;
using Object = UnityEngine.Object;
using Task = System.Threading.Tasks.Task;

public class UIManager : MonoSingleton<UIManager>
{
    [SerializeField] private Transform panelHanger;
    [SerializeField] private Transform popupHanger;
    [SerializeField] private TextMeshProUGUI debugMessageField;

    public Transform PanelHanger => panelHanger;
    public Transform PopupHanger => popupHanger;

    public async Task Initialize()
    {
        MessageBus.Global.Listen<ushort>(MessageChannels.AppErrorReceived, this, OnAppErrorReceived);

        await YooAssetManager.LoadAssetAsync<Object>(
            Consts.YooAssetPackageNameDefault,
            Consts.FontAssetPath);
        await YooAssetManager.LoadAssetAsync<Object>(
            Consts.YooAssetPackageNameDefault,
            Consts.FontSpriteAssetPath);
        GameObject panelPrefab = await YooAssetManager.LoadAssetAsync<GameObject>(
            Consts.YooAssetPackageNameDefault,
            Consts.MainPanelPath
        );
        GameObject panelInstance = GameObject.Instantiate(panelPrefab, PanelHanger);
    }

    private void OnAppErrorReceived(ushort code)
    {
        if (debugMessageField != null)
        {
            debugMessageField.text = $"Error Code: {code}";
        }

        Debug.LogWarning($"[UIManager] App error received: {code}");
    }
    
    
    private void OnDisable()
    {
        MessageBus.Global.UnlistenAll(this);
    }

    private int _currentCharIndex = 0;
    private readonly string[] _loadingChar = new[] { "-", "\\", "|", "/"};
    private float _loadingCharClock = 0.0f;
    private void Update()
    {
        _loadingCharClock -= Time.deltaTime;
        if (_loadingCharClock <= 0.0f)
        {
            _loadingCharClock += 0.5f;
            _currentCharIndex = (_currentCharIndex + 1) %  _loadingChar.Length;
            debugMessageField.text =  _loadingChar[_currentCharIndex];
        }
    }
}
