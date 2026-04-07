using System.Collections.Generic;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

public class PanelBase : MonoBehaviour
{
    #region text field management
    [Header("Managed Text Components")]
    [SerializeField] private List<TextMeshProUGUI> managedTmpTexts = new();

    protected IReadOnlyList<TextMeshProUGUI> ManagedTmpTexts => managedTmpTexts;

    protected virtual void OnTransformChildrenChanged()
    {
        RefreshManagedTexts();
    }

    protected virtual void OnValidate()
    {
        RefreshManagedTexts();
    }

    [ContextMenu("Refresh Managed Texts")]
    public void RefreshManagedTexts()
    {
        CacheManagedTexts();
    }

    private void CacheManagedTexts()
    {
        managedTmpTexts.Clear();
        TextMeshProUGUI[] tmpTexts = GetComponentsInChildren<TextMeshProUGUI>(true);
        for (int i = 0; i < tmpTexts.Length; i++)
        {
            TextMeshProUGUI tmpText = tmpTexts[i];
            if (tmpText != null && tmpText.transform != transform)
            {
                managedTmpTexts.Add(tmpText);
            }
        }
    }

    protected virtual void Awake()
    {
        if (managedTmpTexts.Count == 0)
            CacheManagedTexts();
        RefreshManagerdTextField();
    }

    private async void RefreshManagerdTextField()
    {
        var fontAsset = await YooAssetManager.LoadAssetAsync<TMP_FontAsset>(
            Consts.YooAssetPackageNameDefault,
            Consts.FontAssetPath);
        var spriteAsset = await YooAssetManager.LoadAssetAsync<TMP_SpriteAsset>(
            Consts.YooAssetPackageNameDefault,
            Consts.FontSpriteAssetPath);
        foreach (var text in managedTmpTexts)
        {
            text.font = fontAsset;
            text.spriteAsset = spriteAsset;
        }
    }
    #endregion
}
