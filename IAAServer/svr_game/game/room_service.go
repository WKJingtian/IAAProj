package game

import (
	"context"
	"fmt"
	"time"

	"svr_game/game/model"
)

func (s *Service) GetRoomData(ctx context.Context, openid string) (model.RoomSnapshot, error) {
	playerID, err := s.playerStore.ResolvePlayerID(ctx, openid)
	if err != nil {
		return model.RoomSnapshot{}, err
	}

	roomData, err := s.roomStore.GetOrCreateRoomData(ctx, playerID)
	if err != nil {
		return model.RoomSnapshot{}, err
	}

	return s.buildRoomSnapshot(roomData)
}

func (s *Service) UpgradeFurniture(ctx context.Context, openid string, furnitureID int) (model.RoomSnapshot, model.PlayerData, error) {
	playerID, err := s.playerStore.ResolvePlayerID(ctx, openid)
	if err != nil {
		return model.RoomSnapshot{}, model.PlayerData{}, err
	}

	var updatedPlayer model.PlayerData
	roomData, err := s.roomStore.UpgradeFurniture(
		ctx,
		playerID,
		furnitureID,
		func(cost int, reward int) error {
			playerData, err := s.mutatePlayerData(ctx, openid, func(data *model.PlayerData, now time.Time) (bool, error) {
				if err := s.playerStore.AddCashToPlayerData(data, -cost); err != nil {
					return false, err
				}
				if err := s.playerStore.ApplyRewardToPlayerData(data, 1, reward, now); err != nil {
					if rollbackErr := s.playerStore.AddCashToPlayerData(data, cost); rollbackErr != nil {
						return false, fmt.Errorf("apply furniture reward failed: %w (cash rollback failed: %v)", err, rollbackErr)
					}
					return false, err
				}
				return true, nil
			})
			if err != nil {
				return err
			}
			updatedPlayer = playerData
			return nil
		},
		func(cost int, reward int) error {
			_, err := s.mutatePlayerData(ctx, openid, func(data *model.PlayerData, now time.Time) (bool, error) {
				if err := s.playerStore.AddCashToPlayerData(data, cost); err != nil {
					return false, err
				}
				if err := s.playerStore.ApplyRewardToPlayerData(data, 1, -reward, now); err != nil {
					if revertCashErr := s.playerStore.AddCashToPlayerData(data, -cost); revertCashErr != nil {
						return false, fmt.Errorf("rollback furniture reward failed: %w (cash revert failed: %v)", err, revertCashErr)
					}
					return false, err
				}
				return true, nil
			})
			return err
		},
	)
	if err != nil {
		return model.RoomSnapshot{}, model.PlayerData{}, err
	}

	snapshot, err := s.buildRoomSnapshot(roomData)
	if err != nil {
		return model.RoomSnapshot{}, model.PlayerData{}, err
	}
	return snapshot, updatedPlayer, nil
}

func (s *Service) buildRoomSnapshot(data model.PlayerRoomData) (model.RoomSnapshot, error) {
	if _, ok := s.staticData.Rooms.GetRoom(data.CurrentRoomID); !ok {
		return model.RoomSnapshot{}, fmt.Errorf("current room %d not found", data.CurrentRoomID)
	}

	return model.RoomSnapshot{
		CurrentRoomID:   data.CurrentRoomID,
		FurnitureLevels: append([]int32(nil), data.FurnitureLevels...),
	}, nil
}
