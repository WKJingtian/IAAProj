package staticdata

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"common/applog"
	cconfig "common/config"
)

const (
	ParamsCSVName      = "Data_params.csv"
	ItemsCSVName       = "Data_items.csv"
	EventsCSVName      = "Data_events.csv"
	AssetLevelsCSVName = "Data_assetLevels.csv"
	OptionsCSVName     = "Data_options.csv"
	RoomsCSVName       = "Data_rooms.csv"
	FurnituresCSVName  = "Data_furnitures.csv"
)

func LoadStaticData(dataDir string) (*StaticData, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, fmt.Errorf("data directory cannot be empty")
	}

	paramsRows, err := cconfig.LoadCSV[ParamRow](filepath.Join(dataDir, ParamsCSVName))
	if err != nil {
		return nil, err
	}
	itemsRows, err := cconfig.LoadCSV[ItemRow](filepath.Join(dataDir, ItemsCSVName))
	if err != nil {
		return nil, err
	}
	eventsRows, err := cconfig.LoadCSV[EventRow](filepath.Join(dataDir, EventsCSVName))
	if err != nil {
		return nil, err
	}
	assetLevelRows, err := cconfig.LoadCSV[AssetLevelRow](filepath.Join(dataDir, AssetLevelsCSVName))
	if err != nil {
		return nil, err
	}
	optionRows, err := cconfig.LoadCSV[OptionRow](filepath.Join(dataDir, OptionsCSVName))
	if err != nil {
		return nil, err
	}
	roomRows, err := cconfig.LoadCSV[RoomRow](filepath.Join(dataDir, RoomsCSVName))
	if err != nil {
		return nil, err
	}
	furnitureRows, err := cconfig.LoadCSV[FurnitureRow](filepath.Join(dataDir, FurnituresCSVName))
	if err != nil {
		return nil, err
	}

	paramsByKey, err := indexRows(paramsRows, "params", func(row ParamRow) string {
		return strings.TrimSpace(row.Key)
	})
	if err != nil {
		return nil, err
	}
	itemsByID, err := indexRows(itemsRows, "items", func(row ItemRow) int {
		return row.ID
	})
	if err != nil {
		return nil, err
	}
	eventsByID, err := indexRows(eventsRows, "events", func(row EventRow) EventID {
		return row.ID
	})
	if err != nil {
		return nil, err
	}
	optionsByID, err := indexRows(optionRows, "options", func(row OptionRow) int {
		return row.ID
	})
	if err != nil {
		return nil, err
	}
	roomsByID, err := indexRows(roomRows, "rooms", func(row RoomRow) int {
		return row.ID
	})
	if err != nil {
		return nil, err
	}
	furnituresByID, err := indexRows(furnitureRows, "furnitures", func(row FurnitureRow) int {
		return row.ID
	})
	if err != nil {
		return nil, err
	}
	roomFurnitureIDs, maxRoomID := buildRoomFurnitureIndex(roomRows)

	data := &StaticData{
		Params: ParamsTable{
			Rows:  paramsRows,
			ByKey: paramsByKey,
		},
		Items: ItemsTable{
			Rows: itemsRows,
			ByID: itemsByID,
		},
		Events: EventsTable{
			Rows: eventsRows,
			ByID: eventsByID,
		},
		AssetLevels: AssetLevelsTable{
			Rows: assetLevelRows,
		},
		Options: OptionsTable{
			Rows: optionRows,
			ByID: optionsByID,
		},
		Rooms: RoomsTable{
			Rows:             roomRows,
			ByID:             roomsByID,
			FurnitureRoomIDs: roomFurnitureIDs,
			MaxRoomID:        maxRoomID,
		},
		Furnitures: FurnituresTable{
			Rows: furnitureRows,
			ByID: furnituresByID,
		},
	}

	if err := validateStaticData(data); err != nil {
		return nil, err
	}
	rootRows, err := BuildRootEventRows(data.Events)
	if err != nil {
		return nil, err
	}
	data.Events.RootRows = rootRows
	data.Events.RootRowsByLevel = buildRootEventRowsByLevel(data.Events.RootRows, data.AssetLevels)
	data.Events.TutorialRows = buildTutorialEventRows(data.Events)
	warnTutorialEventConfigs(data.Events)

	return data, nil
}

func validateStaticData(data *StaticData) error {
	if data == nil {
		return fmt.Errorf("static data cannot be nil")
	}

	if err := validateParams(data.Params); err != nil {
		return err
	}
	if err := validateItems(data.Items); err != nil {
		return err
	}
	if err := validateOptions(data.Options); err != nil {
		return err
	}
	if err := validateAssetLevels(data.AssetLevels); err != nil {
		return err
	}
	if err := validateEvents(data.Events, data.Items); err != nil {
		return err
	}
	if err := validateInteractionEventParams(data.Params, data.Events); err != nil {
		return err
	}

	return nil
}

func validateInteractionEventParams(params ParamsTable, events EventsTable) error {
	eventKeys := []string{
		"pirate_blocked_event_id",
		"pirate_hit_event_id",
	}
	for _, key := range eventKeys {
		value, err := params.GetInt64(key)
		if err != nil {
			return err
		}
		if value > 65535 {
			return fmt.Errorf("param %q=%d exceeds max event id 65535", key, value)
		}
		if _, ok := events.ByID[EventID(value)]; !ok {
			return fmt.Errorf("param %q references missing event id %d", key, value)
		}
	}
	return nil
}

func validateParams(table ParamsTable) error {
	if len(table.Rows) == 0 {
		return fmt.Errorf("params table cannot be empty")
	}

	for index, row := range table.Rows {
		if strings.TrimSpace(row.Key) == "" {
			return fmt.Errorf("params row %d has empty key", index+1)
		}
	}

	requiredKeys := []string{
		"event_cost",
		"cash_max",
		"shield_max",
		"start_energy",
		"energy_recover_time",
		"energy_max",
		"max_event_history",
		"pirate_blocked_event_id",
		"pirate_hit_event_id",
		"pirate_hit_cash_loss_percent",
	}
	for _, key := range requiredKeys {
		if _, ok := table.ByKey[key]; !ok {
			return fmt.Errorf("params table missing required key %q", key)
		}
	}
	if _, err := table.GetInt64("event_cost"); err != nil {
		return err
	}
	if _, err := table.GetInt64("start_energy"); err != nil {
		return err
	}
	if _, err := table.GetInt64("energy_recover_time"); err != nil {
		return err
	}
	if _, err := table.GetInt64("energy_max"); err != nil {
		return err
	}
	if _, err := table.GetInt64("max_event_history"); err != nil {
		return err
	}
	if _, err := table.GetInt64("pirate_blocked_event_id"); err != nil {
		return err
	}
	if _, err := table.GetInt64("pirate_hit_event_id"); err != nil {
		return err
	}
	pirateHitCashLossPercent, err := table.GetFloat64("pirate_hit_cash_loss_percent")
	if err != nil {
		return err
	}
	if pirateHitCashLossPercent > 1 {
		return fmt.Errorf("param %q=%f must be within [0,1]", "pirate_hit_cash_loss_percent", pirateHitCashLossPercent)
	}

	return nil
}

func validateItems(table ItemsTable) error {
	for index, row := range table.Rows {
		if row.ID < 0 {
			return fmt.Errorf("items row %d has negative id %d", index+1, row.ID)
		}
	}

	return nil
}

func validateOptions(table OptionsTable) error {
	for index, row := range table.Rows {
		if row.ID < 0 {
			return fmt.Errorf("options row %d has negative id %d", index+1, row.ID)
		}
	}

	return nil
}

func validateAssetLevels(table AssetLevelsTable) error {
	if len(table.Rows) == 0 {
		return fmt.Errorf("asset levels table cannot be empty")
	}

	var prev *AssetLevelRow
	for index := range table.Rows {
		row := table.Rows[index]
		if row.Level < 0 {
			return fmt.Errorf("asset levels row %d has negative level %d", index+1, row.Level)
		}
		if row.MinAsset < 0 {
			return fmt.Errorf("asset levels row %d has negative min_asset %d", index+1, row.MinAsset)
		}
		if prev != nil {
			if row.Level <= prev.Level {
				return fmt.Errorf("asset levels row %d level %d must be greater than previous level %d", index+1, row.Level, prev.Level)
			}
			if row.MinAsset <= prev.MinAsset {
				return fmt.Errorf("asset levels row %d min_asset %d must be greater than previous min_asset %d", index+1, row.MinAsset, prev.MinAsset)
			}
		}
		prev = &row
	}

	return nil
}

func validateEvents(events EventsTable, items ItemsTable) error {
	for index, row := range events.Rows {
		if len(row.RewardID) != len(row.RewardCount) {
			return fmt.Errorf("events row %d reward_id length %d does not match reward_count length %d", index+1, len(row.RewardID), len(row.RewardCount))
		}
		if len(row.ChildrenEvent) != len(row.OptionsOrWeights) {
			return fmt.Errorf("events row %d children_event length %d does not match options_or_weights length %d", index+1, len(row.ChildrenEvent), len(row.OptionsOrWeights))
		}
		if row.MinLevel < 0 || row.MaxLevel < 0 {
			return fmt.Errorf("events row %d has negative level range [%d,%d]", index+1, row.MinLevel, row.MaxLevel)
		}
		if row.MinLevel > row.MaxLevel {
			return fmt.Errorf("events row %d min_level %d cannot be greater than max_level %d", index+1, row.MinLevel, row.MaxLevel)
		}
		for _, childEventID := range row.ChildrenEvent {
			if _, ok := events.ByID[childEventID]; !ok {
				return fmt.Errorf("events row %d references missing child event id %d", index+1, childEventID)
			}
		}
		for _, rewardID := range row.RewardID {
			if _, ok := items.ByID[rewardID]; !ok {
				return fmt.Errorf("events row %d references missing reward item id %d", index+1, rewardID)
			}
		}
	}

	return nil
}

func BuildRootEventRows(events EventsTable) ([]EventRow, error) {
	childIDs := make(map[EventID]struct{})
	for _, row := range events.Rows {
		for _, childID := range row.ChildrenEvent {
			if _, ok := events.ByID[childID]; !ok {
				return nil, fmt.Errorf("events table references missing child event id %d", childID)
			}
			childIDs[childID] = struct{}{}
		}
	}

	rootRows := make([]EventRow, 0, len(events.Rows))
	for _, row := range events.Rows {
		if row.ForTutorial {
			continue
		}
		if _, isChild := childIDs[row.ID]; isChild {
			continue
		}
		rootRows = append(rootRows, row)
	}
	return rootRows, nil
}

func buildTutorialEventRows(events EventsTable) []EventRow {
	tutorialRows := make([]EventRow, 0)
	for _, row := range events.Rows {
		if !row.ForTutorial {
			continue
		}
		tutorialRows = append(tutorialRows, row)
	}

	sort.Slice(tutorialRows, func(i, j int) bool {
		return tutorialRows[i].ID < tutorialRows[j].ID
	})
	return tutorialRows
}

func buildRootEventRowsByLevel(rootRows []EventRow, assetLevels AssetLevelsTable) map[int][]EventRow {
	if len(assetLevels.Rows) == 0 {
		return map[int][]EventRow{}
	}

	rowsByLevel := make(map[int][]EventRow, len(assetLevels.Rows))
	for _, assetLevel := range assetLevels.Rows {
		level := assetLevel.Level
		levelRows := make([]EventRow, 0, len(rootRows))
		for _, row := range rootRows {
			if row.MinLevel > level || row.MaxLevel < level {
				continue
			}
			levelRows = append(levelRows, row)
		}
		rowsByLevel[level] = levelRows
	}

	return rowsByLevel
}

func warnTutorialEventConfigs(events EventsTable) {
	for _, row := range events.Rows {
		if !row.ForTutorial {
			for _, childID := range row.ChildrenEvent {
				child, ok := events.ByID[childID]
				if ok && child.ForTutorial {
					applog.Infof("warning: event %d references tutorial child event %d; tutorial events are ignored by normal event-chain rules", row.ID, childID)
				}
			}
			continue
		}

		if len(row.ChildrenEvent) > 0 {
			applog.Infof("warning: tutorial event %d has children_event configured; tutorial events ignore event-chain rules", row.ID)
		}
		if len(row.OptionsOrWeights) > 0 {
			applog.Infof("warning: tutorial event %d has options_or_weights configured; tutorial events ignore event-chain rules", row.ID)
		}
		if row.AutoProceed {
			applog.Infof("warning: tutorial event %d has auto_proceed=true; tutorial events ignore event-chain rules", row.ID)
		}
		if row.NextIsRandom {
			applog.Infof("warning: tutorial event %d has next_is_random=true; tutorial events ignore event-chain rules", row.ID)
		}
	}
}

func buildRoomFurnitureIndex(roomRows []RoomRow) (map[int]int, int) {
	index := make(map[int]int)
	maxRoomID := 0
	for _, row := range roomRows {
		if row.ID > maxRoomID {
			maxRoomID = row.ID
		}
		for _, furnitureID := range row.Furnitures {
			if _, exists := index[furnitureID]; exists {
				continue
			}
			index[furnitureID] = row.ID
		}
	}
	return index, maxRoomID
}

func indexRows[T any, K comparable](rows []T, tableName string, keyFn func(T) K) (map[K]T, error) {
	index := make(map[K]T, len(rows))
	for _, row := range rows {
		key := keyFn(row)
		if _, exists := index[key]; exists {
			return nil, fmt.Errorf("%s table has duplicate key %v", tableName, key)
		}
		index[key] = row
	}
	return index, nil
}
