package model

import (
	"fmt"
	"strconv"
	"time"

	"svr_game/staticdata"
)

type EventID = staticdata.EventID

type EventChainState struct {
	RootEventID EventID          `bson:"root_event_id"`
	PendingPool map[string]int32 `bson:"pending_pool,omitempty"`
}

type PlayerData struct {
	PlayerID             uint64            `bson:"player_id,omitempty"`
	PirateIncomingFrom   []uint64          `bson:"pirate_incoming_from,omitempty"`
	OpenID               string            `bson:"openid"`
	DebugVal             int64             `bson:"debug_val"`
	Cash                 int32             `bson:"cash,omitempty"`
	Asset                int32             `bson:"asset,omitempty"`
	Energy               int32             `bson:"energy,omitempty"`
	EnergyRecoverAt      time.Time         `bson:"energy_recover_at,omitempty"`
	Shield               int32             `bson:"shield,omitempty"`
	TutorialIndex        int32             `bson:"tutorial_index,omitempty"`
	EventHistory         []EventID         `bson:"event_history,omitempty"`
	EventTargetPlayerIDs []uint64          `bson:"event_target_player_ids,omitempty"`
	ActiveEventChains    []EventChainState `bson:"active_event_chains,omitempty"`
	CreatedAt            time.Time         `bson:"created_at,omitempty"`
	UpdatedAt            time.Time         `bson:"updated_at,omitempty"`
}

func ParseEventID(raw string) (EventID, error) {
	parsed, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return 0, err
	}
	return EventID(parsed), nil
}

func FormatEventID(id EventID) string {
	return strconv.FormatUint(uint64(id), 10)
}

func ParsePendingEventID(raw string) (EventID, error) {
	eventID, err := ParseEventID(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid pending event id %q: %w", raw, err)
	}
	return eventID, nil
}
