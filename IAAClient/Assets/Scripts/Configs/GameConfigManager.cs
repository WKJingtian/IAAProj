using System;
using System.Collections.Generic;
using System.Globalization;
using System.Threading.Tasks;

public static class GameConfigManager
{
    #region data containers

    private static Dictionary<int, EventConfig> _eventConfigs = new();
    private static Dictionary<int, ItemConfig> _itemConfigs = new();
    private static Dictionary<int, RoomConfig> _roomConfigs = new();
    private static Dictionary<int, FurnitureConfig> _furnitureConfigs = new();
    private static Dictionary<string, string> _parameterDict = new();
    private static readonly object _syncRoot = new();
    
    #endregion
    
    #region initialization
    private static bool _initialized = false;
    private static Task _initializeTask;

    public static Task InitializeAsync()
    {
        lock (_syncRoot)
        {
            if (_initialized)
            {
                return Task.CompletedTask;
            }

            if (_initializeTask != null && !_initializeTask.IsFaulted && !_initializeTask.IsCanceled)
            {
                return _initializeTask;
            }

            _initializeTask = InitializeInternalAsync();
            return _initializeTask;
        }
    }

    private static async Task InitializeInternalAsync()
    {
        CsvTableLoader csvLoader = new();

        try
        {
            Dictionary<string, ParamConfig> parameterConfigs = await csvLoader.LoadKeyedTableAsync<ParamConfig, string>(
                Consts.YooAssetPackageNameDefault,
                GlobalConfigs.ParamConfigPath,
                config => config.key
            );
            Dictionary<string, string> parameterDict = new();
            foreach (KeyValuePair<string, ParamConfig> pair in parameterConfigs)
            {
                parameterDict[pair.Key] = pair.Value.value;
            }
            _parameterDict = parameterDict;
            _eventConfigs = await csvLoader.LoadKeyedTableAsync<EventConfig, int>(
                Consts.YooAssetPackageNameDefault,
                GlobalConfigs.EventConfigPath,
                config => config.id
            );
            _itemConfigs = await csvLoader.LoadKeyedTableAsync<ItemConfig, int>(
                Consts.YooAssetPackageNameDefault,
                GlobalConfigs.ItemConfigPath,
                config => config.id
            );
            _roomConfigs = await csvLoader.LoadKeyedTableAsync<RoomConfig, int>(
                Consts.YooAssetPackageNameDefault,
                GlobalConfigs.RoomConfigPath,
                config => config.id
            );
            _furnitureConfigs = await csvLoader.LoadKeyedTableAsync<FurnitureConfig, int>(
                Consts.YooAssetPackageNameDefault,
                GlobalConfigs.FurnitureConfigPath,
                config => config.id
            );

            lock (_syncRoot)
            {
                _initialized = true;
            }
        }
        catch
        {
            lock (_syncRoot)
            {
                _initializeTask = null;
            }
            throw;
        }
    }

    private static void EnsureInitialized()
    {
        if (_initialized)
        {
            return;
        }

        throw new InvalidOperationException(
            "GameConfigManager has not finished initialization. Call and await GameConfigManager.InitializeAsync() first."
        );
    }
    #endregion
    
    #region get config

    public static EventConfig? GetEventConfig(int id)
    {
        EnsureInitialized();
        return _eventConfigs.TryGetValue(id, out EventConfig config) ? config : null;
    }

    public static ItemConfig? GetItemConfig(int id)
    {
        EnsureInitialized();
        return _itemConfigs.TryGetValue(id, out ItemConfig config) ? config : null;
    }

    public static string GetParameterString(string key)
    {
        EnsureInitialized();
        return _parameterDict[key];
    }

    public static RoomConfig? GetRoomConfig(int id)
    {
        EnsureInitialized();
        return _roomConfigs.TryGetValue(id, out RoomConfig config) ? config : null;
    }

    public static FurnitureConfig? GetFurnitureConfig(int id)
    {
        EnsureInitialized();
        return _furnitureConfigs.TryGetValue(id, out FurnitureConfig config) ? config : null;
    }

    public static int GetParameterInt(string key)
    {
        EnsureInitialized();
        string raw = _parameterDict[key];
        if (int.TryParse(raw, NumberStyles.Integer, CultureInfo.InvariantCulture, out int intValue))
        {
            return intValue;
        }

        double doubleValue = double.Parse(raw, CultureInfo.InvariantCulture);
        if (doubleValue < int.MinValue || doubleValue > int.MaxValue || doubleValue != Math.Truncate(doubleValue))
        {
            throw new FormatException($"Parameter {key} value {raw} is not an integer.");
        }
        return (int)doubleValue;
    }

    public static float GetParameterFloat(string key)
    {
        EnsureInitialized();
        return float.Parse(_parameterDict[key], CultureInfo.InvariantCulture);
    }

    public static double GetParameterDouble(string key)
    {
        EnsureInitialized();
        return double.Parse(_parameterDict[key], CultureInfo.InvariantCulture);
    }
    
    #endregion

}
