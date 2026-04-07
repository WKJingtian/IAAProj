package event

import (
	"errors"
	"fmt"
	"time"

	"svr_game/game/model"
)

func (e *Engine) advanceTriggeredCandidate(playerData *PlayerData, candidate triggerCandidate, multiplier int, maxHistory int, now time.Time, ops MutationOps) (EventRow, TriggerResult, error) {
	triggeredEvent := candidate.Event
	rootEventID := candidate.RootEventID

	if candidate.FromChain {
		playerData.ActiveEventChains = removePendingEvent(playerData.ActiveEventChains, rootEventID, triggeredEvent.ID)
	}

	current := triggeredEvent
	result := TriggerResult{
		EventHistoryDelta: make([]EventID, 0, 1),
		TargetPlayerIDs:   make([]uint64, 0, 1),
	}
	for {
		if err := applyScaledRewards(playerData, current, multiplier, now, ops); err != nil {
			return EventRow{}, TriggerResult{}, err
		}
		targetPlayerID, err := applySpecialFlags(playerData, current, ops)
		if err != nil {
			return EventRow{}, TriggerResult{}, err
		}
		result.TargetPlayerIDs = append(result.TargetPlayerIDs, targetPlayerID)
		result.EventHistoryDelta = append(result.EventHistoryDelta, current.ID)
		appendEventHistory(playerData, current.ID, targetPlayerID, maxHistory)

		if len(current.ChildrenEvent) == 0 {
			playerData.ActiveEventChains = removeEventChainIfEmpty(playerData.ActiveEventChains, rootEventID)
			return triggeredEvent, result, nil
		}

		if current.AutoProceed {
			nextEvent, err := e.selectWeightedChildEvent(current)
			if err != nil {
				if errors.Is(err, errNoEligibleChildEvent) {
					playerData.ActiveEventChains = removeEventChainIfEmpty(playerData.ActiveEventChains, rootEventID)
					return triggeredEvent, result, nil
				}
				return EventRow{}, TriggerResult{}, err
			}
			current = nextEvent
			continue
		}

		pool, err := e.buildPendingPool(current)
		if err != nil {
			if errors.Is(err, errNoEligibleChildEvent) {
				playerData.ActiveEventChains = removeEventChainIfEmpty(playerData.ActiveEventChains, rootEventID)
				return triggeredEvent, result, nil
			}
			return EventRow{}, TriggerResult{}, err
		}
		playerData.ActiveEventChains = mergePendingPool(playerData.ActiveEventChains, rootEventID, pool)
		return triggeredEvent, result, nil
	}
}

func applyScaledRewards(playerData *PlayerData, eventRow EventRow, multiplier int, now time.Time, ops MutationOps) error {
	for index, rewardID := range eventRow.RewardID {
		scaledRewardCount, err := scaleInt(eventRow.RewardCount[index], multiplier, fmt.Sprintf("event %d reward_count", eventRow.ID))
		if err != nil {
			return err
		}
		if err := ops.ApplyReward(playerData, rewardID, scaledRewardCount, now); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) selectWeightedChildEvent(parent EventRow) (EventRow, error) {
	if e.staticData == nil {
		return EventRow{}, errors.New("static data is not initialized")
	}
	if len(parent.ChildrenEvent) == 0 {
		return EventRow{}, errors.New("child event list is empty")
	}
	if len(parent.ChildrenEvent) != len(parent.OptionsOrWeights) {
		return EventRow{}, fmt.Errorf("event %d children_event length %d does not match options_or_weights length %d", parent.ID, len(parent.ChildrenEvent), len(parent.OptionsOrWeights))
	}

	weightedEvents := make([]EventRow, 0, len(parent.ChildrenEvent))
	totalWeight := 0
	for index, childID := range parent.ChildrenEvent {
		weight := parent.OptionsOrWeights[index]
		if weight <= 0 {
			continue
		}

		eventRow, ok := e.staticData.Events.ByID[childID]
		if !ok {
			return EventRow{}, fmt.Errorf("event %d references missing child event id %d", parent.ID, childID)
		}
		if eventRow.ForTutorial {
			continue
		}
		eventRow.Weight = weight
		weightedEvents = append(weightedEvents, eventRow)
		totalWeight += weight
	}
	if len(weightedEvents) == 0 {
		return EventRow{}, errNoEligibleChildEvent
	}

	return pickWeightedEvent(e.randIntn, weightedEvents, totalWeight)
}

func pickWeightedEvent(randIntn func(int) int, events []EventRow, totalWeight int) (EventRow, error) {
	if totalWeight <= 0 || len(events) == 0 {
		return EventRow{}, errors.New("no weighted events available")
	}

	roll := randIntn(totalWeight)
	for _, eventRow := range events {
		if roll < eventRow.Weight {
			return eventRow, nil
		}
		roll -= eventRow.Weight
	}

	return EventRow{}, errors.New("weighted event selection failed")
}

func (e *Engine) buildPendingPool(parent EventRow) (map[string]int32, error) {
	children := parent.ChildrenEvent
	weights := parent.OptionsOrWeights
	if len(children) != len(weights) {
		return nil, fmt.Errorf("children_event length %d does not match options_or_weights length %d", len(children), len(weights))
	}

	pool := make(map[string]int32, len(children))
	for index, childID := range children {
		weight := weights[index]
		if weight <= 0 {
			continue
		}

		eventRow, ok := e.staticData.Events.ByID[childID]
		if !ok {
			return nil, fmt.Errorf("event %d references missing child event id %d", parent.ID, childID)
		}
		if eventRow.ForTutorial {
			continue
		}

		key := model.FormatEventID(childID)
		if _, exists := pool[key]; exists {
			return nil, fmt.Errorf("duplicate child event id %d in pending pool", childID)
		}
		pool[key] = int32(weight)
	}
	if len(pool) == 0 {
		return nil, errNoEligibleChildEvent
	}
	return pool, nil
}
