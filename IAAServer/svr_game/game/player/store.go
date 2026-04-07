package player

import (
	"context"
	"errors"
	"sync"
	"time"

	"common/idgen"
	"common/localcache"
	cmongo "common/mongo"
	"svr_game/game/model"
	"svr_game/staticdata"

	driverMongo "go.mongodb.org/mongo-driver/mongo"
)

type Store struct {
	playersCollection *driverMongo.Collection
	playerCache       *localcache.WriteBackCache[model.PlayerData]
	staticData        *staticdata.StaticData
	idGenerator       idgen.Generator
	playerIDsByOpenID sync.Map
}

func NewStore(collectionName string, playerCacheTTL time.Duration, playerFlushInterval time.Duration, staticData *staticdata.StaticData, idGenerator idgen.Generator) (*Store, error) {
	playersCollection, err := getPlayersCollection(collectionName)
	if err != nil {
		return nil, err
	}
	if idGenerator == nil {
		return nil, errors.New("player id generator is required")
	}

	store := &Store{
		playersCollection: playersCollection,
		staticData:        staticData,
		idGenerator:       idGenerator,
	}
	store.playerCache = localcache.NewWriteBackCache(playerCacheTTL, playerFlushInterval, func(ctx context.Context, _ string, data model.PlayerData) error {
		return store.flushPlayerData(ctx, data)
	})

	return store, nil
}

func getPlayersCollection(collectionName string) (*driverMongo.Collection, error) {
	db, err := cmongo.Database()
	if err != nil {
		return nil, err
	}

	return db.Collection(collectionName), nil
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	return s.ensurePlayerIndexes(ctx)
}

func (s *Store) Close(ctx context.Context) error {
	return s.playerCache.Close(ctx)
}

func (s *Store) FlushNow(ctx context.Context) error {
	return s.playerCache.FlushNow(ctx)
}
