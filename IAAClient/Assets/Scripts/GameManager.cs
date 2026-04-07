using UnityEngine;
using System;
using Unity.VisualScripting;

public class GameManager : MonoSingleton<GameManager>
{
    private bool _gameReady = false;
    
    protected override void Awake()
    {
        base.Awake();
        DontDestroyOnLoad(gameObject);
    }

    private async void Start()
    {
        // log in / authentication / load account data
        await Player.Instance.Initialize();
        // load the default asset bundle meta data
        await YooAssetManager.InitializePackageAsync(Consts.YooAssetPackageNameDefault);
        // initialize config data
        await GameConfigManager.InitializeAsync();
        await LocalizationManager.InitializeAsync();
        // instantiate the main panel (which should be in the default bundle)
        await UIManager.Instance.Initialize();
        _gameReady = true;
    }

    private float _tickClock = 0;
    void Update()
    {
        if (!_gameReady) return;
        if (_tickClock > 0)
            _tickClock -= Time.deltaTime;
        else
        {
            _tickClock += 1.0f;
            MessageBus.Global.Dispatch(MessageChannels.GameTickMessage);
        }
    }
}
