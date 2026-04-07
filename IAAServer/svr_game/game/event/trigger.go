package event

import (
	"errors"
	"fmt"
	"time"

	"svr_game/game/model"
	gameplayer "svr_game/game/player"
)

const (
	eventCostParamKey       = "event_cost"
	maxEventHistoryParamKey = "max_event_history"
)

func (e *Engine) Trigger(playerData *PlayerData, now time.Time, multiplier int, ops MutationOps) (TriggerResult, error) {
	if e.staticData == nil {
		return TriggerResult{}, errors.New("static data is not initialized")
	}
	if multiplier <= 0 {
		return TriggerResult{}, fmt.Errorf("multiplier must be greater than 0")
	}

	eventCost, err := e.readRequiredParamInt(eventCostParamKey)
	if err != nil {
		return TriggerResult{}, err
	}
	scaledEventCost, err := scaleInt(eventCost, multiplier, "event_cost")
	if err != nil {
		return TriggerResult{}, err
	}
	maxHistory, err := e.readRequiredParamInt(maxEventHistoryParamKey)
	if err != nil {
		return TriggerResult{}, err
	}

	if int(playerData.Energy) < scaledEventCost {
		if e.cheatModeOn == 1 {
			cheatEnergy, scaleErr := scaleInt(scaledEventCost, 2, "cheat energy")
			if scaleErr != nil {
				return TriggerResult{}, scaleErr
			}
			if err := ops.AddEnergy(playerData, cheatEnergy, now); err != nil {
				return TriggerResult{}, err
			}
		} else {
			return TriggerResult{}, gameplayer.ErrInsufficientEnergy
		}
	}

	tutorialEvent, hasTutorial, err := e.nextTutorialEvent(*playerData)
	if err != nil {
		return TriggerResult{}, err
	}
	if hasTutorial {
		if err := ops.AddEnergy(playerData, -scaledEventCost, now); err != nil {
			return TriggerResult{}, err
		}
		return e.advanceTutorialEvent(playerData, tutorialEvent, multiplier, maxHistory, now, ops)
	}

	candidate, err := e.selectTriggerCandidate(*playerData)
	if err != nil {
		return TriggerResult{}, err
	}
	if err := ops.AddEnergy(playerData, -scaledEventCost, now); err != nil {
		return TriggerResult{}, err
	}

	_, result, err := e.advanceTriggeredCandidate(playerData, candidate, multiplier, maxHistory, now, ops)
	if err != nil {
		return TriggerResult{}, err
	}
	return result, nil
}

func (e *Engine) readRequiredParamInt(key string) (int, error) {
	if e.staticData == nil {
		return 0, errors.New("static data is not initialized")
	}

	value, err := e.staticData.Params.GetInt64(key)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("param %q cannot be negative", key)
	}
	return int(value), nil
}

func appendEventHistory(data *model.PlayerData, eventID model.EventID, targetPlayerID uint64, maxHistory int) {
	if maxHistory <= 0 {
		data.EventHistory = nil
		data.EventTargetPlayerIDs = nil
		return
	}

	data.EventHistory = append(data.EventHistory, eventID)
	data.EventTargetPlayerIDs = append(data.EventTargetPlayerIDs, targetPlayerID)
	if len(data.EventHistory) > maxHistory {
		data.EventHistory = append([]model.EventID(nil), data.EventHistory[len(data.EventHistory)-maxHistory:]...)
		data.EventTargetPlayerIDs = append([]uint64(nil), data.EventTargetPlayerIDs[len(data.EventTargetPlayerIDs)-maxHistory:]...)
	}
}

func scaleInt(value int, multiplier int, fieldName string) (int, error) {
	scaled := int64(value) * int64(multiplier)
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if scaled < minInt || scaled > maxInt {
		return 0, fmt.Errorf("%s overflow after applying multiplier %d", fieldName, multiplier)
	}
	return int(scaled), nil
}
