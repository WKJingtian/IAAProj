using System;
using System.Collections.Generic;
using System.Globalization;
using System.Threading.Tasks;

public static class LocalizationManager
{
    private static readonly List<string> _localizationTablePaths = new()
    {
        "Assets/Configs/Localization_events.csv",
        "Assets/Configs/Localization_items.csv",
        "Assets/Configs/Localization_errors.csv",
        "Assets/Configs/Localization_static.csv",
        "Assets/Configs/Localization_rooms.csv",
        "Assets/Configs/Localization_furnitures.csv",
    };

    private readonly static object _syncRoot = new();
    private static Dictionary<string, Dictionary<string, string>> _localizationTable = new(StringComparer.OrdinalIgnoreCase);
    private static bool _initialized;
    private static Task _initializeTask;

    public static async Task InitializeAsync()
    {
        Task initializeTask;
        lock (_syncRoot)
        {
            if (_initialized)
            {
                return;
            }

            if (_initializeTask != null && !_initializeTask.IsFaulted && !_initializeTask.IsCanceled)
            {
                initializeTask = _initializeTask;
            }
            else
            {
                _initializeTask = InitializeInternalAsync();
                initializeTask = _initializeTask;
            }
        }

        await initializeTask;
    }

    public static string GetLocalizedString(string key)
    {
        EnsureInitialized();
        if (string.IsNullOrWhiteSpace(key))
        {
            return key;
        }

        if (_localizationTable.TryGetValue(key, out var localizedString))
        {
            var language = GlobalConfigs.WeChatUserInfoLanguage;
            if (localizedString.TryGetValue(language, out string value) && !string.IsNullOrEmpty(value))
            {
                return value;
            }
        }

        return key;
    }

    public static string GetLocalizedString(string key, params object[] args)
    {
        string format = GetLocalizedString(key);
        if (args == null || args.Length == 0)
        {
            return format;
        }

        try
        {
            return string.Format(CultureInfo.InvariantCulture, format, args);
        }
        catch (FormatException)
        {
            return format;
        }
    }

    private static async Task InitializeInternalAsync()
    {
        CsvTableLoader csvLoader = new();
        Dictionary<string, Dictionary<string, string>> localizationTable = new(StringComparer.OrdinalIgnoreCase);

        try
        {
            foreach (string localizationTablePath in _localizationTablePaths)
            {
                List<Dictionary<string, string>> rows = await csvLoader.LoadRowsAsync(
                    Consts.YooAssetPackageNameDefault,
                    localizationTablePath
                );
                MergeLocalizationRows(localizationTable, rows, localizationTablePath);
            }

            lock (_syncRoot)
            {
                _localizationTable = localizationTable;
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
            "LocalizationManager has not finished initialization. Call and await LocalizationManager.Instance.InitializeAsync() first."
        );
    }

    private static void MergeLocalizationRows(
        IDictionary<string, Dictionary<string, string>> localizationTable,
        IReadOnlyList<Dictionary<string, string>> rows,
        string location
    )
    {
        for (int rowIndex = 0; rowIndex < rows.Count; rowIndex++)
        {
            IReadOnlyDictionary<string, string> row = rows[rowIndex];
            if (!row.TryGetValue("key", out string key) || string.IsNullOrWhiteSpace(key))
            {
                throw new InvalidOperationException(
                    $"Localization key is missing in {location} at row {rowIndex + 2}."
                );
            }

            Dictionary<string, string> localizedValues = new(StringComparer.OrdinalIgnoreCase);
            foreach (KeyValuePair<string, string> column in row)
            {
                if (string.Equals(column.Key, "key", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                localizedValues[column.Key] = column.Value;
            }

            if (!localizationTable.TryAdd(key, localizedValues))
            {
                throw new InvalidOperationException(
                    $"Duplicate localization key '{key}' found in {location} at row {rowIndex + 2}."
                );
            }
        }
    }
}
