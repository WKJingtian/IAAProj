package event

import (
	"errors"
	"fmt"
	"time"
)

var errNoEligibleChildEvent = errors.New("no eligible child event available")

func (e *Engine) nextTutorialEvent(playerData PlayerData) (EventRow, bool, error) {
	if e.staticData == nil {
		return EventRow{}, false, errors.New("static data is not initialized")
	}

	index := int(playerData.TutorialIndex)
	if index < 0 {
		return EventRow{}, false, fmt.Errorf("tutorial index %d cannot be negative", playerData.TutorialIndex)
	}
	if index >= len(e.staticData.Events.TutorialRows) {
		return EventRow{}, false, nil
	}

	return e.staticData.Events.TutorialRows[index], true, nil
}

func (e *Engine) advanceTutorialEvent(playerData *PlayerData, eventRow EventRow, multiplier int, maxHistory int, now time.Time, ops MutationOps) (TriggerResult, error) {
	if err := applyScaledRewards(playerData, eventRow, multiplier, now, ops); err != nil {
		return TriggerResult{}, err
	}
	targetPlayerID, err := applySpecialFlags(playerData, eventRow, ops)
	if err != nil {
		return TriggerResult{}, err
	}

	appendEventHistory(playerData, eventRow.ID, targetPlayerID, maxHistory)
	playerData.TutorialIndex++
	return TriggerResult{
		EventHistoryDelta: []EventID{eventRow.ID},
		TargetPlayerIDs:   []uint64{targetPlayerID},
	}, nil
}
