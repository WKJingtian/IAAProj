using UnityEngine;

public abstract class MonoSingleton<T> : MonoBehaviour where T : MonoSingleton<T>
{
    private static readonly object SyncRoot = new object();
    private static T _instance;
    private static bool _isApplicationQuitting;
    private bool _isInitialized;

    public static bool HasInstance => _instance != null;

    public static T Instance
    {
        get
        {
            if (_isApplicationQuitting)
            {
                return null;
            }

            lock (SyncRoot)
            {
                if (_instance != null)
                {
                    return _instance;
                }

                _instance = FindObjectOfType<T>();
                if (_instance == null)
                {
                    GameObject obj = new GameObject(typeof(T).Name);
                    _instance = obj.AddComponent<T>();
                }

                _instance.InitializeSingleton();
                return _instance;
            }
        }
    }

    protected virtual bool PersistentAcrossScenes => true;

    protected virtual void OnSingletonInit()
    {
    }

    protected virtual void Awake()
    {
        if (_instance == null)
        {
            _instance = (T)this;
            InitializeSingleton();
            return;
        }

        if (_instance != this)
        {
            Destroy(gameObject);
        }
    }

    protected virtual void OnApplicationQuit()
    {
        _isApplicationQuitting = true;
    }

    protected virtual void OnDestroy()
    {
        if (_instance == this)
        {
            _instance = null;
        }
    }

    private void InitializeSingleton()
    {
        if (_isInitialized)
        {
            return;
        }

        _isInitialized = true;

        if (PersistentAcrossScenes)
        {
            DontDestroyOnLoad(gameObject);
        }

        OnSingletonInit();
    }
}
