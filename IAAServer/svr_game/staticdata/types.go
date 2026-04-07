package staticdata

import (
	"fmt"
	"strconv"
	"strings"
)

type EventID uint16

type ParamRow struct {
	Key   string `csv:"key"`
	Value string `csv:"value"`
}

type ItemRow struct {
	ID    int      `csv:"id"`
	Type  int      `csv:"type"`
	Flags []string `csv:"flags"`
}

type EventRow struct {
	ID               EventID   `csv:"id"`
	RewardID         []int     `csv:"reward_id"`
	RewardCount      []int     `csv:"reward_count"`
	ChildrenEvent    []EventID `csv:"children_event"`
	OptionsOrWeights []int     `csv:"options_or_weights"`
	NextIsRandom     bool      `csv:"next_is_random"`
	ForTutorial      bool      `csv:"for_tutorial"`
	Weight           int       `csv:"weight"`
	AutoProceed      bool      `csv:"auto_proceed"`
	Flags            []string  `csv:"flags"`
	MinLevel         int       `csv:"min_level"`
	MaxLevel         int       `csv:"max_level"`
}

type AssetLevelRow struct {
	Level                  int     `csv:"level"`
	MinAsset               int64   `csv:"min_asset"`
	NormalEventRewardMult  float64 `csv:"normal_event_reward_mult"`
	RaidEventRewardMult    float64 `csv:"raid_event_reward_mult"`
	LotteryEventRewardMult float64 `csv:"lottery_event_reward_mult"`
}

type OptionRow struct {
	ID    int `csv:"id"`
	Style int `csv:"style"`
}

type RoomRow struct {
	ID         int    `csv:"id"`
	Furnitures []int  `csv:"furnitures"`
	Prefab     string `csv:"prefab"`
}

type FurnitureRow struct {
	ID                     int    `csv:"id"`
	UpgradeCost            []int  `csv:"upgrade_cost"`
	Key                    string `csv:"key"`
	FurnitureUpgradeReward []int  `csv:"furniture_upgrade_reward"`
}

type ParamsTable struct {
	Rows  []ParamRow
	ByKey map[string]ParamRow
}

type ItemsTable struct {
	Rows []ItemRow
	ByID map[int]ItemRow
}

type EventsTable struct {
	Rows            []EventRow
	ByID            map[EventID]EventRow
	RootRows        []EventRow
	RootRowsByLevel map[int][]EventRow
	TutorialRows    []EventRow
}

type AssetLevelsTable struct {
	Rows []AssetLevelRow
}

type OptionsTable struct {
	Rows []OptionRow
	ByID map[int]OptionRow
}

type RoomsTable struct {
	Rows             []RoomRow
	ByID             map[int]RoomRow
	FurnitureRoomIDs map[int]int
	MaxRoomID        int
}

type FurnituresTable struct {
	Rows []FurnitureRow
	ByID map[int]FurnitureRow
}

type StaticData struct {
	Params      ParamsTable
	Items       ItemsTable
	Events      EventsTable
	AssetLevels AssetLevelsTable
	Options     OptionsTable
	Rooms       RoomsTable
	Furnitures  FurnituresTable
}

func (t ParamsTable) GetRow(key string) (ParamRow, bool) {
	row, ok := t.ByKey[key]
	return row, ok
}

func (t ParamsTable) GetString(key string) (string, bool) {
	row, ok := t.GetRow(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(row.Value), true
}

func (t ParamsTable) GetInt64(key string) (int64, error) {
	value, ok := t.GetString(key)
	if !ok {
		return 0, fmt.Errorf("param %q not found", key)
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return parsed, nil
	}

	floatParsed, floatErr := strconv.ParseFloat(value, 64)
	if floatErr != nil {
		return 0, fmt.Errorf("parse param %q as int64 failed: %w", key, err)
	}
	if floatParsed != float64(int64(floatParsed)) {
		return 0, fmt.Errorf("param %q=%q is not an integer", key, value)
	}
	return int64(floatParsed), nil
}

func (t ParamsTable) GetBool(key string) (bool, error) {
	value, ok := t.GetString(key)
	if !ok {
		return false, fmt.Errorf("param %q not found", key)
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse param %q as bool failed: %w", key, err)
	}
	return parsed, nil
}

func (t ParamsTable) GetFloat64(key string) (float64, error) {
	value, ok := t.GetString(key)
	if !ok {
		return 0, fmt.Errorf("param %q not found", key)
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse param %q as float64 failed: %w", key, err)
	}
	return parsed, nil
}

func (t AssetLevelsTable) MatchLevelByAsset(asset int32) int {
	if len(t.Rows) == 0 {
		return 0
	}

	assetValue := int64(asset)
	left := 0
	right := len(t.Rows) - 1
	matched := 0

	for left <= right {
		mid := left + (right-left)/2
		row := t.Rows[mid]
		if row.MinAsset <= assetValue {
			matched = row.Level
			left = mid + 1
			continue
		}
		right = mid - 1
	}

	return matched
}

func (t AssetLevelsTable) MinAssetForLevelGreaterThan(level int) (int64, bool) {
	for _, row := range t.Rows {
		if row.Level > level {
			return row.MinAsset, true
		}
	}
	return 0, false
}

func (t RoomsTable) GetRoom(id int) (RoomRow, bool) {
	row, ok := t.ByID[id]
	return row, ok
}

func (t RoomsTable) NextRoomID(id int) (int, bool) {
	found := false
	nextID := 0
	for _, row := range t.Rows {
		if row.ID <= id {
			continue
		}
		if !found || row.ID < nextID {
			nextID = row.ID
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return nextID, true
}

func (t RoomsTable) RoomIDByFurnitureID(furnitureID int) (int, bool) {
	roomID, ok := t.FurnitureRoomIDs[furnitureID]
	return roomID, ok
}
