package game

import (
	"context"
	"math/rand"
	"time"

	"common/idgen"
	"svr_game/game/event"
	"svr_game/game/player"
	"svr_game/game/room"
	"svr_game/staticdata"
)

type Service struct {
	playerStore *player.Store
	eventEngine *event.Engine
	roomStore   *room.Store
	staticData  *staticdata.StaticData
}

func NewService(collectionName string, playerCacheTTL time.Duration, playerFlushInterval time.Duration, staticData *staticdata.StaticData, cheatModeOn int, playerIDGenerator idgen.Generator) (*Service, error) {
	playerStore, err := player.NewStore(collectionName, playerCacheTTL, playerFlushInterval, staticData, playerIDGenerator)
	if err != nil {
		return nil, err
	}
	roomStore, err := room.NewStore(collectionName, staticData)
	if err != nil {
		return nil, err
	}

	return &Service{
		playerStore: playerStore,
		eventEngine: event.NewEngine(staticData, rand.Intn, cheatModeOn),
		roomStore:   roomStore,
		staticData:  staticData,
	}, nil
}

func (s *Service) EnsureIndexes(ctx context.Context) error {
	if err := s.playerStore.EnsureIndexes(ctx); err != nil {
		return err
	}
	return s.roomStore.EnsureIndexes(ctx)
}

func (s *Service) Close(ctx context.Context) error {
	return s.playerStore.Close(ctx)
}

func (s *Service) FlushNow(ctx context.Context) error {
	return s.playerStore.FlushNow(ctx)
}
