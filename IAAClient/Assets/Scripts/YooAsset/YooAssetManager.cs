using System;
using System.Collections.Generic;
using System.Reflection;
using System.Threading.Tasks;
using UnityEngine;
using YooAsset;

public static class YooAssetManager
{
    private const int AssetLoadTimeoutSeconds = 30;
    private static readonly MethodInfo YooAssetsUpdateMethod = typeof(YooAssets).GetMethod(
        "Update",
        BindingFlags.NonPublic | BindingFlags.Static
    );
    private static readonly IRemoteServices FixedRemoteServicesInstance = new FixedRemoteServices();
    private static readonly object SyncRoot = new();
    private static readonly Dictionary<string, Task<ResourcePackage>> PackageInitializeTasks = new();
    private static readonly HashSet<string> RefreshedPackagesThisSession = new();
    private static readonly Dictionary<string, AssetHandle> CachedAssetHandles = new();
    private static readonly Dictionary<string, Task<AssetHandle>> AssetLoadTasks = new();
    private static int LastManualTickFrame = -1;

    // initialize package by name
    public static Task<ResourcePackage> InitializePackageAsync(
        string packageName,
        int requestVersionTimeoutSeconds = 30,
        int manifestTimeoutSeconds = 30
    )
    {
        ResourcePackage package = GetOrCreatePackage(packageName);

        lock (SyncRoot)
        {
            bool refreshedThisSession = RefreshedPackagesThisSession.Contains(packageName);
            // if this package is already initialized for this run, just return the cached result
            if (package.InitializeStatus == EOperationStatus.Succeed && refreshedThisSession)
            {
                YooAssets.SetDefaultPackage(package);
                return Task.FromResult(package);
            }

            // if any initialization task exists
            if (PackageInitializeTasks.TryGetValue(packageName, out Task<ResourcePackage> initializeTask))
            {
                // not completed yet
                if (!initializeTask.IsCompleted)
                {
                    return initializeTask;
                }

                // task finished, but result is not shown in RefreshedPackagesThisSession.
                // this might mean the last initialization failed unexpectedly, so retry now
                PackageInitializeTasks.Remove(packageName);
            }

            Task<ResourcePackage> newInitializeTask = InitializePackageInternalAsync(
                package,
                requestVersionTimeoutSeconds,
                manifestTimeoutSeconds,
                !refreshedThisSession
            );
            // actual loading process
            PackageInitializeTasks[packageName] = newInitializeTask;
            _ = newInitializeTask.ContinueWith(
                _ =>
                {
                    lock (SyncRoot)
                    {
                        if (
                            PackageInitializeTasks.TryGetValue(packageName, out Task<ResourcePackage> currentTask)
                            && ReferenceEquals(currentTask, newInitializeTask)
                        )
                        {
                            PackageInitializeTasks.Remove(packageName);
                        }
                    }
                },
                TaskScheduler.Default
            );
            return newInitializeTask;
        }
    }

    public static Task<TAsset> LoadAsset<TAsset>(string packageName, string location)
        where TAsset : UnityEngine.Object
    {
        return LoadAssetAsync<TAsset>(packageName, location);
    }

    // this is the primary way to load a resource
    public static async Task<TAsset> LoadAssetAsync<TAsset>(string packageName, string location)
        where TAsset : UnityEngine.Object
    {
        // load the package first
        await InitializePackageAsync(packageName);

        string assetKey = GetAssetKey<TAsset>(packageName, location);
        Task<TAsset> resultTask = null;

        lock (SyncRoot)
        {
            // if this result is already in the cache
            if (CachedAssetHandles.TryGetValue(assetKey, out AssetHandle cachedHandle) && cachedHandle.IsValid)
            {
                return cachedHandle.GetAssetObject<TAsset>();
            }

            // similar to package loading process
            if (AssetLoadTasks.TryGetValue(assetKey, out Task<AssetHandle> loadTask))
            {
                if (!loadTask.IsCompleted)
                {
                    resultTask = GetAssetObjectAsync<TAsset>(assetKey, loadTask);
                }
                else
                {
                    AssetLoadTasks.Remove(assetKey);
                }
            }

            // actual loading process here
            if (resultTask == null)
            {
                Task<AssetHandle> newLoadTask = LoadAssetHandleAsync<TAsset>(packageName, location);
                AssetLoadTasks[assetKey] = newLoadTask;
                _ = newLoadTask.ContinueWith(
                    _ =>
                    {
                        lock (SyncRoot)
                        {
                            if (
                                AssetLoadTasks.TryGetValue(assetKey, out Task<AssetHandle> currentTask)
                                && ReferenceEquals(currentTask, newLoadTask)
                            )
                            {
                                AssetLoadTasks.Remove(assetKey);
                            }
                        }
                    },
                    TaskScheduler.Default
                );
                resultTask = GetAssetObjectAsync<TAsset>(assetKey, newLoadTask);
            }
        }

        return await resultTask;
    }

    // unload package, never use this on default package!
    public static async Task UnloadPackageAssetsAsync(string packageName)
    {
        ResourcePackage package = YooAssets.TryGetPackage(packageName);
        if (package == null)
        {
            return;
        }

        ClearPackageHandles(packageName);

        UnloadAllAssetsOperation operation = package.UnloadAllAssetsAsync(
            new UnloadAllAssetsOptions
            {
                ReleaseAllHandles = true,
                LockLoadOperation = true
            }
        );
        await WaitUntilDoneAsync(
            operation,
            AssetLoadTimeoutSeconds,
            $"unload package assets {packageName}",
            $"Unload package assets failed. package={packageName}"
        );
    }

    // unload a single asset handle
    public static void ReleaseAsset<TAsset>(string packageName, string location)
        where TAsset : UnityEngine.Object
    {
        string assetKey = GetAssetKey<TAsset>(packageName, location);
        ReleaseAssetInternal(assetKey);
    }

    public static void ReleaseAllAssets()
    {
        AssetHandle[] handles;
        lock (SyncRoot)
        {
            handles = new AssetHandle[CachedAssetHandles.Count];
            CachedAssetHandles.Values.CopyTo(handles, 0);
            CachedAssetHandles.Clear();
        }

        foreach (AssetHandle handle in handles)
        {
            if (handle.IsValid)
            {
                handle.Release();
            }
        }
    }

    // returns a ready asset handle
    private static async Task<AssetHandle> LoadAssetHandleAsync<TAsset>(string packageName, string location)
        where TAsset : UnityEngine.Object
    {
        ResourcePackage package = await InitializePackageAsync(packageName);
        AssetHandle handle = package.LoadAssetAsync<TAsset>(location);
        await WaitUntilDoneAsync(
            handle,
            AssetLoadTimeoutSeconds,
            $"load asset {location}",
            $"YooAsset load asset timeout. package={packageName}, location={location}"
        );
        if (handle.Status != EOperationStatus.Succeed)
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] load failed: {packageName} | {location} | {handle.LastError}");
            throw new InvalidOperationException(
                "YooAsset load asset failed. package="
                + packageName
                + ", location="
                + location
                + ", error="
                + handle.LastError
            );
        }
        return handle;
    }

    // internal method that communicates with yoo asset
    private static async Task<ResourcePackage> InitializePackageInternalAsync(
        ResourcePackage package,
        int requestVersionTimeoutSeconds,
        int manifestTimeoutSeconds,
        bool forceRemoteRefresh
    )
    {
        if (forceRemoteRefresh && package.InitializeStatus == EOperationStatus.Succeed)
        {
            package = await RecreatePackageAsync(package);
        }

        if (package.InitializeStatus != EOperationStatus.Succeed)
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] init start: {package.PackageName}");
            InitializationOperation initializeOperation = package.InitializeAsync(CreateInitializeParameters(package.PackageName));
            await WaitUntilDoneAsync(
                initializeOperation,
                requestVersionTimeoutSeconds,
                $"initialize package {package.PackageName}",
                $"YooAsset package initialize timeout. package={package.PackageName}"
            );
            if (initializeOperation.Status != EOperationStatus.Succeed)
            {
                MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] init failed: {package.PackageName} | {initializeOperation.Error}");
                throw new InvalidOperationException(
                    "YooAsset package initialize failed. package="
                    + package.PackageName
                    + ", error="
                    + initializeOperation.Error
                );
            }
        }

        if (forceRemoteRefresh)
        {
            await RefreshPackageManifestAsync(package, requestVersionTimeoutSeconds, manifestTimeoutSeconds);
            lock (SyncRoot)
            {
                RefreshedPackagesThisSession.Add(package.PackageName);
            }
        }

        YooAssets.SetDefaultPackage(package);
        return package;
    }

    // force a package to update if local version code does not match the remote one
    private static async Task RefreshPackageManifestAsync(
        ResourcePackage package,
        int requestVersionTimeoutSeconds,
        int manifestTimeoutSeconds
    )
    {
        string currentVersion = GetCurrentPackageVersion(package);

        MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] refresh manifest start: {package.PackageName}");
        RequestPackageVersionOperation requestVersionOperation = package.RequestPackageVersionAsync(
            true,
            requestVersionTimeoutSeconds
        );
        await WaitUntilDoneAsync(
            requestVersionOperation,
            requestVersionTimeoutSeconds,
            $"request version {package.PackageName}",
            $"YooAsset request package version timeout. package={package.PackageName}"
        );
        if (requestVersionOperation.Status != EOperationStatus.Succeed)
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] request version failed: {package.PackageName} | {requestVersionOperation.Error}");
            throw new InvalidOperationException(
                "YooAsset request package version failed. package="
                + package.PackageName
                + ", error="
                + requestVersionOperation.Error
            );
        }
        string remoteVersion = requestVersionOperation.PackageVersion?.Trim();

        if (string.IsNullOrWhiteSpace(remoteVersion))
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] request version empty: {package.PackageName}");
            throw new InvalidOperationException("YooAsset returned empty package version. package=" + package.PackageName);
        }

        if (!string.IsNullOrEmpty(currentVersion) && !string.Equals(currentVersion, remoteVersion, StringComparison.Ordinal))
        {
            await ResetPackageRuntimeAsync(package);
        }

        UpdatePackageManifestOperation updateManifestOperation = package.UpdatePackageManifestAsync(
            remoteVersion,
            manifestTimeoutSeconds
        );
        await WaitUntilDoneAsync(
            updateManifestOperation,
            manifestTimeoutSeconds,
            $"update manifest {package.PackageName}",
            $"YooAsset update package manifest timeout. package={package.PackageName}, version={remoteVersion}"
        );
        if (updateManifestOperation.Status != EOperationStatus.Succeed)
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] update manifest failed: {package.PackageName} | {updateManifestOperation.Error}");
            throw new InvalidOperationException(
                "YooAsset update package manifest failed. package="
                + package.PackageName
                + ", version="
                + remoteVersion
                + ", error="
                + updateManifestOperation.Error
            );
        }
    }

    private static async Task<ResourcePackage> RecreatePackageAsync(ResourcePackage package)
    {
        string packageName = package.PackageName;

        ClearPackageHandles(packageName);

        DestroyOperation destroyOperation = package.DestroyAsync();
        await WaitUntilDoneAsync(
            destroyOperation,
            AssetLoadTimeoutSeconds,
            $"destroy package {packageName}",
            $"YooAsset destroy package timeout. package={packageName}"
        );
        if (destroyOperation.Status != EOperationStatus.Succeed)
        {
            MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] destroy package failed: {packageName} | {destroyOperation.Error}");
            throw new InvalidOperationException(
                "YooAsset destroy package failed. package="
                + packageName
                + ", error="
                + destroyOperation.Error
            );
        }

        lock (SyncRoot)
        {
            YooAssets.RemovePackage(package);
            package = YooAssets.CreatePackage(packageName);
        }

        return package;
    }

    private static async Task<TAsset> GetAssetObjectAsync<TAsset>(string assetKey, Task<AssetHandle> loadTask)
        where TAsset : UnityEngine.Object
    {
        AssetHandle handle = await loadTask;

        lock (SyncRoot)
        {
            CachedAssetHandles[assetKey] = handle;
        }

        return handle.GetAssetObject<TAsset>();
    }

    private static string GetCurrentPackageVersion(ResourcePackage package)
    {
        try
        {
            return package.GetPackageVersion();
        }
        catch
        {
            return string.Empty;
        }
    }

    private static async Task ResetPackageRuntimeAsync(ResourcePackage package)
    {
        ClearPackageHandles(package.PackageName);

        UnloadAllAssetsOperation operation = package.UnloadAllAssetsAsync(
            new UnloadAllAssetsOptions
            {
                ReleaseAllHandles = true,
                LockLoadOperation = true
            }
        );
        await WaitUntilDoneAsync(
            operation,
            AssetLoadTimeoutSeconds,
            $"clear runtime cache {package.PackageName}",
            $"YooAsset clear runtime cache timeout. package={package.PackageName}"
        );
    }

    private static ResourcePackage GetOrCreatePackage(string packageName)
    {
        lock (SyncRoot)
        {
            EnsureYooAssetsInitialized();

            ResourcePackage package = YooAssets.TryGetPackage(packageName);
            return package ?? YooAssets.CreatePackage(packageName);
        }
    }

    private static void EnsureYooAssetsInitialized()
    {
        if (!YooAssets.Initialized)
        {
            YooAssets.Initialize();
        }
    }

    private static string GetAssetKey<TAsset>(string packageName, string location)
        where TAsset : UnityEngine.Object
    {
        return packageName + "|" + typeof(TAsset).FullName + "|" + location;
    }

    private static async Task WaitUntilDoneAsync(
        AsyncOperationBase operation,
        int timeoutSeconds,
        string stageLabel,
        string timeoutMessage
    )
    {
        DateTime deadline = DateTime.UtcNow.AddSeconds(timeoutSeconds);
        while (!operation.IsDone)
        {
            TickYooAssetsIfNeeded();

            if (DateTime.UtcNow >= deadline)
            {
                MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] timeout: {stageLabel}");
                throw new TimeoutException(timeoutMessage);
            }

            await Task.Yield();
        }
    }

    private static async Task WaitUntilDoneAsync(
        HandleBase handle,
        int timeoutSeconds,
        string stageLabel,
        string timeoutMessage
    )
    {
        DateTime deadline = DateTime.UtcNow.AddSeconds(timeoutSeconds);
        while (!handle.IsDone)
        {
            TickYooAssetsIfNeeded();

            if (DateTime.UtcNow >= deadline)
            {
                MessageBus.Global.Dispatch(MessageChannels.UIDisplayDebugMessage, $"[YooAsset] timeout: {stageLabel}");
                throw new TimeoutException(timeoutMessage);
            }

            await Task.Yield();
        }
    }

    private static void TickYooAssetsIfNeeded()
    {
        if (LastManualTickFrame == Time.frameCount)
        {
            return;
        }

        LastManualTickFrame = Time.frameCount;
        YooAssetsUpdateMethod?.Invoke(null, null);
    }

    private static void ReleaseAssetInternal(string assetKey)
    {
        AssetHandle handle = null;
        lock (SyncRoot)
        {
            if (!CachedAssetHandles.TryGetValue(assetKey, out handle))
            {
                return;
            }

            CachedAssetHandles.Remove(assetKey);
        }

        if (handle.IsValid)
        {
            handle.Release();
        }
    }

    private static void ClearPackageHandles(string packageName)
    {
        List<string> removeKeys = new();
        List<AssetHandle> removeHandles = new();

        lock (SyncRoot)
        {
            foreach (KeyValuePair<string, AssetHandle> pair in CachedAssetHandles)
            {
                if (!pair.Key.StartsWith(packageName + "|"))
                {
                    continue;
                }

                removeKeys.Add(pair.Key);
                removeHandles.Add(pair.Value);
            }

            foreach (string key in removeKeys)
            {
                CachedAssetHandles.Remove(key);
                AssetLoadTasks.Remove(key);
            }
        }

        foreach (AssetHandle handle in removeHandles)
        {
            if (handle.IsValid)
            {
                handle.Release();
            }
        }
    }

    private static InitializeParameters CreateInitializeParameters(string _)
    {
        WebPlayModeParameters parameters = new();
        string packageRoot = $"{WeChatWASM.WX.env.USER_DATA_PATH}/__GAME_FILE_CACHE";
#if UNITY_WEBGL && WEIXINMINIGAME
        parameters.WebServerFileSystemParameters = WechatFileSystemCreater.CreateFileSystemParameters(
            packageRoot,
            FixedRemoteServicesInstance
        );
#endif
        return parameters;
    }

    private sealed class FixedRemoteServices : IRemoteServices
    {
        string IRemoteServices.GetRemoteMainURL(string fileName)
        {
            return $"{GlobalConfigs.YooAssetResourceRootUrl.TrimEnd('/')}/{fileName}";
        }

        string IRemoteServices.GetRemoteFallbackURL(string fileName)
        {
            return $"{GlobalConfigs.YooAssetResourceRootUrl.TrimEnd('/')}/{fileName}";
        }
    }
}
