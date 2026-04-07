package game

import (
	"context"
	"time"

	"svr_game/game/event"
	"svr_game/game/model"
)

func (s *Service) TriggerEvent(ctx context.Context, openid string, multiplier int) ([]model.EventID, []uint64, model.PlayerData, error) {
	var historyDelta []model.EventID
	var targetPlayerIDs []uint64
	data, err := s.mutatePlayerData(ctx, openid, func(playerData *model.PlayerData, now time.Time) (bool, error) {
		result, err := s.eventEngine.Trigger(playerData, now, multiplier, event.MutationOps{
			AddEnergy:   s.playerStore.AddEnergyToPlayerData,
			ApplyReward: s.playerStore.ApplyRewardToPlayerData,
			ApplyPirateIncoming: func(sourcePlayerID uint64) (uint64, bool, error) {
				return s.playerStore.ApplyPirateIncoming(ctx, sourcePlayerID)
			},
		})
		if err != nil {
			return false, err
		}
		historyDelta = append([]model.EventID(nil), result.EventHistoryDelta...)
		targetPlayerIDs = append([]uint64(nil), result.TargetPlayerIDs...)
		return true, nil
	})
	if err != nil {
		return nil, nil, model.PlayerData{}, err
	}

	return historyDelta, targetPlayerIDs, data, nil
}
