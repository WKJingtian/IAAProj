package room

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	cmongo "common/mongo"
	"svr_game/game/model"
	"svr_game/staticdata"

	"go.mongodb.org/mongo-driver/bson"
	driverMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PlayerRoomData = model.PlayerRoomData

type Store struct {
	roomsCollection *driverMongo.Collection
	staticData      *staticdata.StaticData
	playerLocks     sync.Map
}

func NewStore(collectionName string, staticData *staticdata.StaticData) (*Store, error) {
	if staticData == nil {
		return nil, errors.New("static data is required")
	}

	roomsCollection, err := getRoomsCollection(collectionName)
	if err != nil {
		return nil, err
	}

	return &Store{
		roomsCollection: roomsCollection,
		staticData:      staticData,
	}, nil
}

func getRoomsCollection(collectionName string) (*driverMongo.Collection, error) {
	db, err := cmongo.Database()
	if err != nil {
		return nil, err
	}

	return db.Collection(collectionName + "_rooms"), nil
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.roomsCollection.Indexes().CreateOne(indexCtx, driverMongo.IndexModel{
		Keys: bson.D{{Key: "player_id", Value: 1}},
		Options: options.Index().
			SetName("idx_player_room_player_id_unique").
			SetUnique(true).
			SetPartialFilterExpression(bson.M{
				"player_id": bson.M{"$gt": 0},
			}),
	})
	if err != nil {
		return fmt.Errorf("create player_room player_id index failed: %w", err)
	}

	return nil
}

func (s *Store) GetOrCreateRoomData(ctx context.Context, playerID uint64) (PlayerRoomData, error) {
	var result PlayerRoomData
	err := s.withPlayerLock(playerID, func() error {
		now := time.Now().UTC()
		data, err := s.getOrCreateRoomDataLocked(ctx, playerID, now)
		if err != nil {
			return err
		}
		result = data
		return nil
	})
	if err != nil {
		return PlayerRoomData{}, err
	}
	return result, nil
}

func (s *Store) UpgradeFurniture(ctx context.Context, playerID uint64, furnitureID int, apply func(cost int, reward int) error, rollback func(cost int, reward int) error) (PlayerRoomData, error) {
	if apply == nil {
		return PlayerRoomData{}, errors.New("apply callback is required")
	}

	var result PlayerRoomData
	err := s.withPlayerLock(playerID, func() error {
		now := time.Now().UTC()
		data, err := s.getOrCreateRoomDataLocked(ctx, playerID, now)
		if err != nil {
			return err
		}

		roomRow, furnitureIndex, furnitureRow, currentLevel, err := s.prepareUpgrade(data, furnitureID)
		if err != nil {
			return err
		}

		cost := furnitureRow.UpgradeCost[currentLevel]
		reward := furnitureRow.FurnitureUpgradeReward[currentLevel]
		if err := apply(cost, reward); err != nil {
			return err
		}

		data.FurnitureLevels[furnitureIndex]++
		completed, err := s.isRoomCompleted(data, roomRow)
		if err != nil {
			if rollback != nil {
				if rollbackErr := rollback(cost, reward); rollbackErr != nil {
					return fmt.Errorf("rollback failed after room completion check error: %w (rollback failed: %v)", err, rollbackErr)
				}
			}
			return err
		}
		if completed {
			if nextRoomID, ok := s.staticData.Rooms.NextRoomID(data.CurrentRoomID); ok {
				nextRoom, ok := s.staticData.Rooms.GetRoom(nextRoomID)
				if !ok {
					if rollback != nil {
						if rollbackErr := rollback(cost, reward); rollbackErr != nil {
							return fmt.Errorf("rollback failed after missing next room: %w (rollback failed: %v)", errCurrentRoomNotFound(nextRoomID), rollbackErr)
						}
					}
					return errCurrentRoomNotFound(nextRoomID)
				}
				data.CurrentRoomID = nextRoomID
				data.FurnitureLevels = make([]int32, len(nextRoom.Furnitures))
			}
		}
		data.UpdatedAt = now

		if err := s.flushRoomData(ctx, data); err != nil {
			if rollback != nil {
				if rollbackErr := rollback(cost, reward); rollbackErr != nil {
					return fmt.Errorf("flush room data failed: %w (rollback failed: %v)", err, rollbackErr)
				}
			}
			return fmt.Errorf("flush room data failed: %w", err)
		}

		result = data
		return nil
	})
	if err != nil {
		return PlayerRoomData{}, err
	}
	return result, nil
}

func (s *Store) withPlayerLock(playerID uint64, fn func() error) error {
	if playerID == 0 {
		return errors.New("player id cannot be zero")
	}

	lockValue, _ := s.playerLocks.LoadOrStore(playerID, &sync.Mutex{})
	mutex := lockValue.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	return fn()
}

func (s *Store) getOrCreateRoomDataLocked(ctx context.Context, playerID uint64, now time.Time) (PlayerRoomData, error) {
	data, found, err := s.findRoomData(ctx, playerID)
	if err != nil {
		return PlayerRoomData{}, err
	}
	if !found {
		data, err = s.newDefaultRoomData(playerID, now)
		if err != nil {
			return PlayerRoomData{}, err
		}
		if err := s.flushRoomData(ctx, data); err != nil {
			return PlayerRoomData{}, err
		}
		return data, nil
	}

	changed, err := s.normalizeRoomData(&data, playerID, now)
	if err != nil {
		return PlayerRoomData{}, err
	}
	if changed {
		if err := s.flushRoomData(ctx, data); err != nil {
			return PlayerRoomData{}, err
		}
	}
	return data, nil
}

func (s *Store) findRoomData(ctx context.Context, playerID uint64) (PlayerRoomData, bool, error) {
	var doc PlayerRoomData
	err := s.roomsCollection.FindOne(ctx, bson.M{"player_id": playerID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, driverMongo.ErrNoDocuments) {
			return PlayerRoomData{}, false, nil
		}
		return PlayerRoomData{}, false, err
	}
	return doc, true, nil
}

func (s *Store) newDefaultRoomData(playerID uint64, now time.Time) (PlayerRoomData, error) {
	roomRow, err := s.firstRoomRow()
	if err != nil {
		return PlayerRoomData{}, err
	}

	return PlayerRoomData{
		PlayerID:        playerID,
		CurrentRoomID:   roomRow.ID,
		FurnitureLevels: make([]int32, len(roomRow.Furnitures)),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (s *Store) normalizeRoomData(data *PlayerRoomData, playerID uint64, now time.Time) (bool, error) {
	changed := false
	if data.PlayerID == 0 {
		data.PlayerID = playerID
		changed = true
	}
	if data.CreatedAt.IsZero() {
		defaultData, err := s.newDefaultRoomData(playerID, now)
		if err != nil {
			return false, err
		}
		data.CurrentRoomID = defaultData.CurrentRoomID
		data.FurnitureLevels = defaultData.FurnitureLevels
		data.CreatedAt = defaultData.CreatedAt
		data.UpdatedAt = defaultData.UpdatedAt
		return true, nil
	}

	roomRow, ok := s.staticData.Rooms.GetRoom(data.CurrentRoomID)
	if !ok {
		return false, errCurrentRoomNotFound(data.CurrentRoomID)
	}
	if len(data.FurnitureLevels) != len(roomRow.Furnitures) {
		aligned := make([]int32, len(roomRow.Furnitures))
		copy(aligned, data.FurnitureLevels)
		data.FurnitureLevels = aligned
		changed = true
	}
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = now
		changed = true
	}

	return changed, nil
}

func (s *Store) prepareUpgrade(data PlayerRoomData, furnitureID int) (staticdata.RoomRow, int, staticdata.FurnitureRow, int, error) {
	roomRow, ok := s.staticData.Rooms.GetRoom(data.CurrentRoomID)
	if !ok {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, errCurrentRoomNotFound(data.CurrentRoomID)
	}

	furnitureRoomID, ok := s.staticData.Rooms.RoomIDByFurnitureID(furnitureID)
	if !ok {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, ErrFurnitureNotFound
	}
	if furnitureRoomID != data.CurrentRoomID {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, ErrFurnitureNotInCurrentRoom
	}

	furnitureIndex := -1
	for index, id := range roomRow.Furnitures {
		if id == furnitureID {
			furnitureIndex = index
			break
		}
	}
	if furnitureIndex < 0 {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, ErrFurnitureNotInCurrentRoom
	}

	furnitureRow, ok := s.staticData.Furnitures.ByID[furnitureID]
	if !ok {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, ErrFurnitureNotFound
	}

	currentLevel := int(data.FurnitureLevels[furnitureIndex])
	if currentLevel < 0 {
		currentLevel = 0
	}
	if currentLevel >= len(furnitureRow.UpgradeCost) {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, ErrFurnitureMaxLevel
	}
	if currentLevel >= len(furnitureRow.FurnitureUpgradeReward) {
		return staticdata.RoomRow{}, 0, staticdata.FurnitureRow{}, 0, fmt.Errorf("furniture %d reward config is shorter than upgrade config", furnitureID)
	}

	return roomRow, furnitureIndex, furnitureRow, currentLevel, nil
}

func (s *Store) isRoomCompleted(data PlayerRoomData, roomRow staticdata.RoomRow) (bool, error) {
	for index, furnitureID := range roomRow.Furnitures {
		furnitureRow, ok := s.staticData.Furnitures.ByID[furnitureID]
		if !ok {
			return false, ErrFurnitureNotFound
		}
		if index >= len(data.FurnitureLevels) {
			return false, nil
		}
		if int(data.FurnitureLevels[index]) < len(furnitureRow.UpgradeCost) {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) flushRoomData(ctx context.Context, data PlayerRoomData) error {
	if data.PlayerID == 0 {
		return errors.New("room data player_id cannot be zero")
	}

	update := bson.M{
		"$set": bson.M{
			"current_room_id":  data.CurrentRoomID,
			"furniture_levels": data.FurnitureLevels,
			"updated_at":       data.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"player_id":  data.PlayerID,
			"created_at": data.CreatedAt,
		},
	}

	_, err := s.roomsCollection.UpdateOne(
		ctx,
		bson.M{"player_id": data.PlayerID},
		update,
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) firstRoomRow() (staticdata.RoomRow, error) {
	if len(s.staticData.Rooms.Rows) == 0 {
		return staticdata.RoomRow{}, errors.New("rooms table is empty")
	}

	first := s.staticData.Rooms.Rows[0]
	for _, row := range s.staticData.Rooms.Rows[1:] {
		if row.ID < first.ID {
			first = row
		}
	}
	return first, nil
}

func errCurrentRoomNotFound(roomID int) error {
	return fmt.Errorf("current room %d not found", roomID)
}
