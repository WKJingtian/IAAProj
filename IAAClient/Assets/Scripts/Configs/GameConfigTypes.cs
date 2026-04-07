
using System.Collections.Generic;

public struct ParamConfig
{
    public string key;
    public string value;
}

public struct EventConfig
{
    public int id;
    public List<int> reward_id;
    public List<int> reward_count;
    public List<int> children_event;
    public List<int> options_or_weights;
    public bool next_is_random;
    public bool for_tutorial;
    public bool auto_proceed;
    public int weight;
    public int min_level;
    public int max_level;
    public List<string> flags;
}

public enum ItemType : int
{
    NOTYPE = 0,
    CURRENCY = 1,
}

public struct ItemConfig
{
    public int id;
    public ItemType type;
    public List<string> flags;
}

public struct RoomConfig
{
    public int id;
    public List<int> furnitures;
    public string prefab;
}

public struct FurnitureConfig
{
    public int id;
    public List<int> upgrade_cost;
    public string key;
    public List<int> furniture_upgrade_reward;
}
