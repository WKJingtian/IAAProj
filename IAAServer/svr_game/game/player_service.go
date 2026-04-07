package game

import (
	"context"
	"time"

	"svr_game/game/model"
)

func (s *Service) GetPlayerData(ctx context.Context, openid string) (model.PlayerData, error) {
	return s.playerStore.GetPlayerData(ctx, openid)
}

func (s *Service) IncrementDebugVal(ctx context.Context, openid string) (model.PlayerData, error) {
	return s.playerStore.IncrementDebugVal(ctx, openid)
}

func (s *Service) GetOrCreatePlayerData(ctx context.Context, openid string) (model.PlayerData, error) {
	return s.playerStore.GetOrCreatePlayerData(ctx, openid)
}

func (s *Service) ResolvePlayerID(ctx context.Context, openid string) (uint64, error) {
	return s.playerStore.ResolvePlayerID(ctx, openid)
}

func (s *Service) mutatePlayerData(ctx context.Context, openid string, mutate func(*model.PlayerData, time.Time) (bool, error)) (model.PlayerData, error) {
	return s.playerStore.MutatePlayerData(ctx, openid, mutate)
}
