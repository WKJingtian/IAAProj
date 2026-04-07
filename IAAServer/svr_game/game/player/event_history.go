package player

import (
	"fmt"
	"math"
)

const maxEventHistoryParamKey = "max_event_history"

func (s *Store) readRequiredEventIDParam(key string) (EventID, error) {
	value, err := s.readRequiredParamInt(key)
	if err != nil {
		return 0, err
	}
	if value > math.MaxUint16 {
		return 0, fmt.Errorf("param %q=%d exceeds max event id %d", key, value, math.MaxUint16)
	}
	return EventID(value), nil
}

func appendEventHistoryEntry(data *PlayerData, eventID EventID, targetPlayerID uint64, maxHistory int) {
	if maxHistory <= 0 {
		data.EventHistory = nil
		data.EventTargetPlayerIDs = nil
		return
	}

	data.EventHistory = append(data.EventHistory, eventID)
	data.EventTargetPlayerIDs = append(data.EventTargetPlayerIDs, targetPlayerID)
	if len(data.EventHistory) > maxHistory {
		data.EventHistory = append([]EventID(nil), data.EventHistory[len(data.EventHistory)-maxHistory:]...)
		data.EventTargetPlayerIDs = append([]uint64(nil), data.EventTargetPlayerIDs[len(data.EventTargetPlayerIDs)-maxHistory:]...)
	}
}
