package player

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const pirateTargetMinLevelParamKey = "pirate_target_min_level"

type pirateTargetCandidate struct {
	OpenID   string `bson:"openid"`
	PlayerID uint64 `bson:"player_id"`
}

func (s *Store) ApplyPirateIncoming(ctx context.Context, sourcePlayerID uint64) (uint64, bool, error) {
	if sourcePlayerID == 0 {
		return 0, false, fmt.Errorf("source player id cannot be zero")
	}

	requiredMinLevel, err := s.readRequiredParamInt(pirateTargetMinLevelParamKey)
	if err != nil {
		return 0, false, err
	}
	requiredMinAsset, ok := s.staticData.AssetLevels.MinAssetForLevelGreaterThan(requiredMinLevel)
	if !ok {
		return 0, false, nil
	}

	findCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cursor, err := s.playersCollection.Find(findCtx, bson.M{
		"player_id": bson.M{"$ne": sourcePlayerID, "$gt": 0},
		"asset":     bson.M{"$gte": requiredMinAsset},
	}, options.Find().SetProjection(bson.M{
		"openid":    1,
		"player_id": 1,
	}))
	if err != nil {
		return 0, false, err
	}
	defer cursor.Close(findCtx)

	candidates := make([]pirateTargetCandidate, 0, 8)
	for cursor.Next(findCtx) {
		var candidate pirateTargetCandidate
		if err := cursor.Decode(&candidate); err != nil {
			return 0, false, err
		}
		if candidate.OpenID == "" || candidate.PlayerID == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if err := cursor.Err(); err != nil {
		return 0, false, err
	}
	if len(candidates) == 0 {
		return 0, false, nil
	}

	target := candidates[rand.Intn(len(candidates))]
	if _, err := s.MutatePlayerData(ctx, target.OpenID, func(data *PlayerData, _ time.Time) (bool, error) {
		data.PirateIncomingFrom = append(data.PirateIncomingFrom, sourcePlayerID)
		return true, nil
	}); err != nil {
		return 0, false, err
	}

	s.rememberPlayerID(target.OpenID, target.PlayerID)
	return target.PlayerID, true, nil
}
