package player

import (
	"context"
	"errors"
	"fmt"
	"time"

	"svr_game/game/model"

	"go.mongodb.org/mongo-driver/bson"
	driverMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PlayerData = model.PlayerData
type EventID = model.EventID
type EventChainState = model.EventChainState

func (s *Store) newDefaultPlayerData(openid string, now time.Time) (PlayerData, error) {
	playerID, err := s.nextPlayerID()
	if err != nil {
		return PlayerData{}, err
	}
	startEnergy, energyMax, recoverTime, err := s.readEnergyParams()
	if err != nil {
		return PlayerData{}, err
	}

	initialEnergy := startEnergy
	if initialEnergy > energyMax {
		initialEnergy = energyMax
	}

	data := PlayerData{
		PlayerID:             playerID,
		PirateIncomingFrom:   []uint64{},
		OpenID:               openid,
		DebugVal:             0,
		Cash:                 0,
		Asset:                0,
		Energy:               int32(initialEnergy),
		Shield:               0,
		TutorialIndex:        0,
		EventHistory:         []EventID{},
		EventTargetPlayerIDs: []uint64{},
		ActiveEventChains:    []EventChainState{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if initialEnergy < energyMax && recoverTime > 0 {
		data.EnergyRecoverAt = now.Add(time.Duration(recoverTime) * time.Second)
	}
	return data, nil
}

func (s *Store) ensurePlayerIndexes(ctx context.Context) error {
	indexCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.playersCollection.Indexes().CreateOne(indexCtx, driverMongo.IndexModel{
		Keys: bson.D{{Key: "openid", Value: 1}},
		Options: options.Index().
			SetName("idx_openid_unique").
			SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create player index failed: %w", err)
	}

	_, err = s.playersCollection.Indexes().CreateOne(indexCtx, driverMongo.IndexModel{
		Keys: bson.D{{Key: "player_id", Value: 1}},
		Options: options.Index().
			SetName("idx_player_id_unique").
			SetUnique(true).
			SetPartialFilterExpression(bson.M{
				"player_id": bson.M{"$gt": 0},
			}),
	})
	if err != nil {
		return fmt.Errorf("create player_id index failed: %w", err)
	}

	_, err = s.playersCollection.Indexes().CreateOne(indexCtx, driverMongo.IndexModel{
		Keys: bson.D{{Key: "asset", Value: 1}},
		Options: options.Index().
			SetName("idx_asset"),
	})
	if err != nil {
		return fmt.Errorf("create asset index failed: %w", err)
	}

	return nil
}

func (s *Store) findPlayerData(ctx context.Context, openid string) (PlayerData, bool, error) {
	var doc PlayerData
	err := s.playersCollection.FindOne(ctx, bson.M{"openid": openid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, driverMongo.ErrNoDocuments) {
			return PlayerData{}, false, nil
		}
		return PlayerData{}, false, err
	}
	s.rememberPlayerID(doc.OpenID, doc.PlayerID)

	return doc, true, nil
}

func (s *Store) loadPlayerData(ctx context.Context, openid string) (PlayerData, error) {
	doc, found, err := s.findPlayerData(ctx, openid)
	if err != nil {
		return PlayerData{}, err
	}
	if found {
		return doc, nil
	}

	return PlayerData{
		OpenID:   openid,
		DebugVal: 0,
	}, nil
}

func (s *Store) GetPlayerData(ctx context.Context, openid string) (PlayerData, error) {
	now := time.Now().UTC()
	if cached, ok := s.playerCache.Get(openid, now); ok {
		s.rememberPlayerID(cached.OpenID, cached.PlayerID)
		return cached, nil
	}

	loaded, err := s.loadPlayerData(ctx, openid)
	if err != nil {
		return PlayerData{}, err
	}

	stored := s.playerCache.StoreLoaded(openid, loaded, now)
	s.rememberPlayerID(stored.OpenID, stored.PlayerID)
	return stored, nil
}

func (s *Store) IncrementDebugVal(ctx context.Context, openid string) (PlayerData, error) {
	return s.MutatePlayerData(ctx, openid, func(data *PlayerData, _ time.Time) (bool, error) {
		data.DebugVal++
		return true, nil
	})
}

func (s *Store) GetOrCreatePlayerData(ctx context.Context, openid string) (PlayerData, error) {
	return s.MutatePlayerData(ctx, openid, func(data *PlayerData, _ time.Time) (bool, error) {
		return s.settlePirateIncoming(data)
	})
}

func (s *Store) ResolvePlayerID(ctx context.Context, openid string) (uint64, error) {
	if playerID, ok := s.cachedPlayerID(openid); ok {
		return playerID, nil
	}

	data, err := s.GetOrCreatePlayerData(ctx, openid)
	if err != nil {
		return 0, err
	}
	return data.PlayerID, nil
}

func (s *Store) nextPlayerID() (uint64, error) {
	if s.idGenerator == nil {
		return 0, errors.New("player id generator is not initialized")
	}
	return s.idGenerator.NextID()
}

func (s *Store) flushPlayerData(ctx context.Context, data PlayerData) error {
	if data.OpenID == "" {
		return errors.New("player openid cannot be empty")
	}

	createdAt := data.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := data.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	update := bson.M{
		"$set": bson.M{
			"player_id":               data.PlayerID,
			"debug_val":               data.DebugVal,
			"cash":                    data.Cash,
			"asset":                   data.Asset,
			"energy":                  data.Energy,
			"energy_recover_at":       data.EnergyRecoverAt,
			"shield":                  data.Shield,
			"tutorial_index":          data.TutorialIndex,
			"event_history":           data.EventHistory,
			"event_target_player_ids": data.EventTargetPlayerIDs,
			"active_event_chains":     data.ActiveEventChains,
			"pirate_incoming_from":    data.PirateIncomingFrom,
			"updated_at":              updatedAt,
		},
		"$setOnInsert": bson.M{
			"openid":     data.OpenID,
			"created_at": createdAt,
		},
	}

	_, err := s.playersCollection.UpdateOne(
		ctx,
		bson.M{"openid": data.OpenID},
		update,
		options.Update().SetUpsert(true),
	)
	s.rememberPlayerID(data.OpenID, data.PlayerID)
	return err
}
