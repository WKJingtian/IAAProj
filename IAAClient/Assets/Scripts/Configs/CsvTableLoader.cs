using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.Reflection;
using System.Text;
using System.Threading.Tasks;
using UnityEngine;

public sealed class CsvTableLoader
{
    public async Task<Dictionary<TKey, TConfig>> LoadKeyedTableAsync<TConfig, TKey>(
        string packageName,
        string configFilepath,
        Func<TConfig, TKey> keySelector
    )
    {
        if (keySelector == null)
        {
            throw new ArgumentNullException(nameof(keySelector));
        }

        List<Dictionary<string, string>> rows = await LoadRowsAsync(packageName, configFilepath);
        Dictionary<TKey, TConfig> result = new(rows.Count);
        for (int rowIndex = 0; rowIndex < rows.Count; rowIndex++)
        {
            TConfig config = ConvertRow<TConfig>(rows[rowIndex], configFilepath, rowIndex + 2);
            TKey key = keySelector(config);
            if (!result.TryAdd(key, config))
            {
                throw new InvalidOperationException(
                    $"Duplicate config key '{key}' found in {configFilepath} at row {rowIndex + 2}."
                );
            }
        }

        return result;
    }

    public async Task<List<Dictionary<string, string>>> LoadRowsAsync(string packageName, string configFilepath)
    {
        TextAsset csvAsset = null;
        try
        {
            csvAsset = await YooAssetManager.LoadAssetAsync<TextAsset>(packageName, configFilepath);
            if (csvAsset == null)
            {
                throw new InvalidOperationException($"Failed to load CSV asset at location: {configFilepath}");
            }

            return ParseCsv(csvAsset.text, configFilepath);
        }
        finally
        {
            if (csvAsset != null)
            {
                YooAssetManager.ReleaseAsset<TextAsset>(packageName, configFilepath);
            }
        }
    }

    private static List<Dictionary<string, string>> ParseCsv(string content, string location)
    {
        List<List<string>> parsedRows = ParseCsvRows(content);
        List<List<string>> nonEmptyRows = new(parsedRows.Count);
        foreach (List<string> row in parsedRows)
        {
            if (!IsRowEmpty(row))
            {
                nonEmptyRows.Add(row);
            }
        }

        if (nonEmptyRows.Count == 0)
        {
            throw new InvalidOperationException($"CSV file is empty: {location}");
        }

        List<string> headers = nonEmptyRows[0];
        if (headers.Count == 0)
        {
            throw new InvalidOperationException($"CSV file has no headers: {location}");
        }

        headers[0] = headers[0].TrimStart('\uFEFF');
        for (int headerIndex = 0; headerIndex < headers.Count; headerIndex++)
        {
            headers[headerIndex] = headers[headerIndex].Trim();
        }

        List<Dictionary<string, string>> rows = new(nonEmptyRows.Count - 1);
        for (int rowIndex = 1; rowIndex < nonEmptyRows.Count; rowIndex++)
        {
            List<string> values = nonEmptyRows[rowIndex];
            if (values.Count != headers.Count)
            {
                throw new InvalidOperationException(
                    $"CSV column count mismatch in {location} at row {rowIndex + 1}. Expected {headers.Count}, got {values.Count}."
                );
            }

            Dictionary<string, string> row = new(StringComparer.OrdinalIgnoreCase);
            for (int columnIndex = 0; columnIndex < headers.Count; columnIndex++)
            {
                row[headers[columnIndex]] = values[columnIndex].Trim();
            }

            rows.Add(row);
        }

        return rows;
    }

    private static List<List<string>> ParseCsvRows(string content)
    {
        string normalizedContent = content.Replace("\r\n", "\n").Replace('\r', '\n');
        List<List<string>> rows = new();
        List<string> currentRow = new();
        StringBuilder currentValue = new();
        bool inQuotes = false;
        int bracketDepth = 0;

        for (int index = 0; index < normalizedContent.Length; index++)
        {
            char c = normalizedContent[index];
            if (c == '"')
            {
                if (inQuotes && index + 1 < normalizedContent.Length && normalizedContent[index + 1] == '"')
                {
                    currentValue.Append('"');
                    index++;
                    continue;
                }

                inQuotes = !inQuotes;
                continue;
            }

            if (!inQuotes)
            {
                switch (c)
                {
                    case '[':
                        bracketDepth++;
                        break;
                    case ']':
                        if (bracketDepth > 0)
                        {
                            bracketDepth--;
                        }
                        break;
                    case ',':
                        if (bracketDepth == 0)
                        {
                            currentRow.Add(currentValue.ToString());
                            currentValue.Clear();
                            continue;
                        }
                        break;
                    case '\n':
                        if (bracketDepth == 0)
                        {
                            currentRow.Add(currentValue.ToString());
                            currentValue.Clear();
                            rows.Add(currentRow);
                            currentRow = new List<string>();
                            continue;
                        }
                        break;
                }
            }

            currentValue.Append(c);
        }

        currentRow.Add(currentValue.ToString());
        rows.Add(currentRow);
        return rows;
    }

    private static bool IsRowEmpty(IReadOnlyList<string> row)
    {
        for (int index = 0; index < row.Count; index++)
        {
            if (!string.IsNullOrWhiteSpace(row[index]))
            {
                return false;
            }
        }

        return true;
    }

    private static TConfig ConvertRow<TConfig>(
        IReadOnlyDictionary<string, string> row,
        string location,
        int rowNumber
    )
    {
        object boxedConfig = Activator.CreateInstance(typeof(TConfig));

        foreach (FieldInfo field in typeof(TConfig).GetFields(BindingFlags.Instance | BindingFlags.Public))
        {
            if (!row.TryGetValue(field.Name, out string rawValue))
            {
                continue;
            }

            try
            {
                field.SetValue(boxedConfig, ConvertValue(field.FieldType, rawValue));
            }
            catch (Exception ex)
            {
                throw new InvalidOperationException(
                    $"Failed to parse field '{field.Name}' in {location} at row {rowNumber}. Raw value: '{rawValue}'.",
                    ex
                );
            }
        }

        foreach (PropertyInfo property in typeof(TConfig).GetProperties(BindingFlags.Instance | BindingFlags.Public))
        {
            if (!property.CanWrite || !row.TryGetValue(property.Name, out string rawValue))
            {
                continue;
            }

            try
            {
                property.SetValue(boxedConfig, ConvertValue(property.PropertyType, rawValue));
            }
            catch (Exception ex)
            {
                throw new InvalidOperationException(
                    $"Failed to parse property '{property.Name}' in {location} at row {rowNumber}. Raw value: '{rawValue}'.",
                    ex
                );
            }
        }

        return (TConfig)boxedConfig;
    }

    private static object ConvertValue(Type targetType, string rawValue)
    {
        Type nullableType = Nullable.GetUnderlyingType(targetType);
        if (nullableType != null)
        {
            return string.IsNullOrWhiteSpace(rawValue) ? null : ConvertValue(nullableType, rawValue);
        }

        if (targetType == typeof(string))
        {
            return Unquote(rawValue.Trim());
        }

        if (targetType == typeof(bool))
        {
            string normalized = rawValue.Trim();
            if (bool.TryParse(normalized, out bool boolValue))
            {
                return boolValue;
            }

            if (normalized == "1")
            {
                return true;
            }

            if (normalized == "0")
            {
                return false;
            }

            throw new FormatException($"Unsupported boolean literal: {rawValue}");
        }

        if (targetType.IsEnum)
        {
            string normalized = rawValue.Trim();
            if (int.TryParse(normalized, NumberStyles.Integer, CultureInfo.InvariantCulture, out int enumInt))
            {
                return Enum.ToObject(targetType, enumInt);
            }

            return Enum.Parse(targetType, normalized, true);
        }

        if (targetType == typeof(int))
        {
            return int.Parse(rawValue.Trim(), CultureInfo.InvariantCulture);
        }

        if (targetType == typeof(float))
        {
            return float.Parse(rawValue.Trim(), CultureInfo.InvariantCulture);
        }

        if (targetType == typeof(double))
        {
            return double.Parse(rawValue.Trim(), CultureInfo.InvariantCulture);
        }

        if (targetType.IsGenericType && targetType.GetGenericTypeDefinition() == typeof(List<>))
        {
            return ConvertListValue(targetType.GetGenericArguments()[0], rawValue);
        }

        throw new NotSupportedException($"Unsupported CSV target type: {targetType.FullName}");
    }

    private static object ConvertListValue(Type elementType, string rawValue)
    {
        Type listType = typeof(List<>).MakeGenericType(elementType);
        IList list = (IList)Activator.CreateInstance(listType);

        string trimmed = rawValue.Trim();
        if (string.IsNullOrEmpty(trimmed) || trimmed == "[]")
        {
            return list;
        }

        if (trimmed.StartsWith("[", StringComparison.Ordinal) && trimmed.EndsWith("]", StringComparison.Ordinal))
        {
            trimmed = trimmed.Substring(1, trimmed.Length - 2);
        }

        if (string.IsNullOrWhiteSpace(trimmed))
        {
            return list;
        }

        List<string> values = SplitDelimitedLine(trimmed);
        foreach (string value in values)
        {
            list.Add(ConvertValue(elementType, value));
        }

        return list;
    }

    private static List<string> SplitDelimitedLine(string line)
    {
        List<string> values = new();
        int segmentStart = 0;
        int bracketDepth = 0;
        bool inQuotes = false;

        for (int index = 0; index < line.Length; index++)
        {
            char c = line[index];
            switch (c)
            {
                case '"':
                    if (inQuotes && index + 1 < line.Length && line[index + 1] == '"')
                    {
                        index++;
                    }
                    else
                    {
                        inQuotes = !inQuotes;
                    }
                    break;
                case '[':
                    if (!inQuotes)
                    {
                        bracketDepth++;
                    }
                    break;
                case ']':
                    if (!inQuotes && bracketDepth > 0)
                    {
                        bracketDepth--;
                    }
                    break;
                case ',':
                    if (!inQuotes && bracketDepth == 0)
                    {
                        values.Add(line.Substring(segmentStart, index - segmentStart));
                        segmentStart = index + 1;
                    }
                    break;
            }
        }

        values.Add(line.Substring(segmentStart));
        return values;
    }

    private static string Unquote(string value)
    {
        if (value.Length >= 2 && value[0] == '"' && value[value.Length - 1] == '"')
        {
            return value.Substring(1, value.Length - 2).Replace("\"\"", "\"");
        }

        return value;
    }
}
