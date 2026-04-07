package event

import (
	"errors"
	"fmt"

	"svr_game/game/model"
)

type triggerCandidate struct {
	Event       EventRow
	Weight      int
	RootEventID EventID
	FromChain   bool
}

func (e *Engine) selectTriggerCandidate(playerData PlayerData) (triggerCandidate, error) {
	candidates, totalWeight, err := e.buildTriggerCandidates(playerData)
	if err != nil {
		return triggerCandidate{}, err
	}
	if totalWeight <= 0 || len(candidates) == 0 {
		return triggerCandidate{}, errors.New("no trigger candidates available")
	}

	roll := e.randIntn(totalWeight)
	for _, candidate := range candidates {
		if roll < candidate.Weight {
			return candidate, nil
		}
		roll -= candidate.Weight
	}

	return triggerCandidate{}, errors.New("trigger candidate selection failed")
}

func (e *Engine) buildTriggerCandidates(playerData PlayerData) ([]triggerCandidate, int, error) {
	if e.staticData == nil {
		return nil, 0, errors.New("static data is not initialized")
	}

	playerAssetLevel := e.staticData.AssetLevels.MatchLevelByAsset(playerData.Asset)
	rootRows := e.staticData.Events.RootRowsByLevel[playerAssetLevel]

	candidates := make([]triggerCandidate, 0, len(rootRows)+len(playerData.ActiveEventChains))
	totalWeight := 0

	for _, eventRow := range rootRows {
		if eventRow.Weight <= 0 || hasEventChain(playerData.ActiveEventChains, eventRow.ID) {
			continue
		}

		candidates = append(candidates, triggerCandidate{
			Event:       eventRow,
			Weight:      eventRow.Weight,
			RootEventID: eventRow.ID,
		})
		totalWeight += eventRow.Weight
	}

	for _, chain := range playerData.ActiveEventChains {
		for rawID, weight32 := range chain.PendingPool {
			if weight32 <= 0 {
				continue
			}

			eventID, err := model.ParsePendingEventID(rawID)
			if err != nil {
				return nil, 0, err
			}
			eventRow, ok := e.staticData.Events.ByID[eventID]
			if !ok {
				return nil, 0, fmt.Errorf("pending event references missing event id %d", eventID)
			}
			if eventRow.ForTutorial {
				continue
			}

			weight := int(weight32)
			candidates = append(candidates, triggerCandidate{
				Event:       eventRow,
				Weight:      weight,
				RootEventID: chain.RootEventID,
				FromChain:   true,
			})
			totalWeight += weight
		}
	}

	return candidates, totalWeight, nil
}
