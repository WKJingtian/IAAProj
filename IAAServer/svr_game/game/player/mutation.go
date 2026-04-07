package player

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	startEnergyParamKey       = "start_energy"
	energyRecoverTimeParamKey = "energy_recover_time"
	energyMaxParamKey         = "energy_max"
)

var ErrInsufficientEnergy = errors.New("not enough energy")
var ErrInsufficientCash = errors.New("not enough cash")

func IsInsufficientEnergy(err error) bool {
	return errors.Is(err, ErrInsufficientEnergy)
}

func IsInsufficientCash(err error) bool {
	return errors.Is(err, ErrInsufficientCash)
}

func (s *Store) readRequiredParamInt(key string) (int, error) {
	if s.staticData == nil {
		return 0, errors.New("static data is not initialized")
	}

	value, err := s.staticData.Params.GetInt64(key)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("param %q cannot be negative", key)
	}
	return int(value), nil
}

func (s *Store) readRequiredParamFloat(key string) (float64, error) {
	if s.staticData == nil {
		return 0, errors.New("static data is not initialized")
	}

	value, err := s.staticData.Params.GetFloat64(key)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("param %q cannot be negative", key)
	}
	return value, nil
}

func (s *Store) MutatePlayerData(ctx context.Context, openid string, mutate func(*PlayerData, time.Time) (bool, error)) (PlayerData, error) {
	now := time.Now().UTC()

	if cached, ok, err := s.playerCache.MutateIfPresent(openid, now, func(data *PlayerData) (bool, error) {
		changed, err := s.normalizePlayerData(data, openid, now)
		if err != nil {
			return false, err
		}
		refreshed, err := s.refreshPlayerEnergy(data, now)
		if err != nil {
			return false, err
		}
		mutated, err := mutate(data, now)
		if err != nil {
			return false, err
		}
		if changed || refreshed || mutated {
			data.UpdatedAt = now
			return true, nil
		}
		return false, nil
	}); ok {
		if err != nil {
			return PlayerData{}, err
		}
		return cached, nil
	}

	loaded, err := s.loadPlayerData(ctx, openid)
	if err != nil {
		return PlayerData{}, err
	}

	data, err := s.playerCache.MutateWithLoaded(openid, loaded, now, func(doc *PlayerData) (bool, error) {
		changed, err := s.normalizePlayerData(doc, openid, now)
		if err != nil {
			return false, err
		}
		refreshed, err := s.refreshPlayerEnergy(doc, now)
		if err != nil {
			return false, err
		}
		mutated, err := mutate(doc, now)
		if err != nil {
			return false, err
		}
		if changed || refreshed || mutated {
			doc.UpdatedAt = now
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return PlayerData{}, err
	}

	return data, nil
}

func (s *Store) normalizePlayerData(data *PlayerData, openid string, now time.Time) (bool, error) {
	changed := false

	if data.OpenID == "" {
		data.OpenID = openid
		changed = true
	}
	if data.TutorialIndex < 0 {
		data.TutorialIndex = 0
		changed = true
	}
	if data.CreatedAt.IsZero() {
		defaultData, err := s.newDefaultPlayerData(openid, now)
		if err != nil {
			return false, err
		}
		data.DebugVal = defaultData.DebugVal
		data.Cash = defaultData.Cash
		data.Asset = defaultData.Asset
		data.Energy = defaultData.Energy
		data.EnergyRecoverAt = defaultData.EnergyRecoverAt
		data.Shield = defaultData.Shield
		data.TutorialIndex = defaultData.TutorialIndex
		if data.EventHistory == nil {
			data.EventHistory = defaultData.EventHistory
		}
		if data.EventTargetPlayerIDs == nil {
			data.EventTargetPlayerIDs = defaultData.EventTargetPlayerIDs
		}
		if data.ActiveEventChains == nil {
			data.ActiveEventChains = defaultData.ActiveEventChains
		}
		data.PlayerID = defaultData.PlayerID
		data.CreatedAt = defaultData.CreatedAt
		data.UpdatedAt = defaultData.UpdatedAt
		changed = true
	}
	if data.PlayerID == 0 {
		playerID, err := s.nextPlayerID()
		if err != nil {
			return false, err
		}
		data.PlayerID = playerID
		changed = true
	}
	s.rememberPlayerID(data.OpenID, data.PlayerID)
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = now
		changed = true
	}
	if len(data.EventTargetPlayerIDs) != len(data.EventHistory) {
		data.EventTargetPlayerIDs = alignEventTargetPlayerIDs(data.EventTargetPlayerIDs, len(data.EventHistory))
		changed = true
	}
	return changed, nil
}

func alignEventTargetPlayerIDs(targets []uint64, expectedLen int) []uint64 {
	if expectedLen <= 0 {
		return nil
	}
	if len(targets) == expectedLen {
		return targets
	}
	if len(targets) > expectedLen {
		return append([]uint64(nil), targets[len(targets)-expectedLen:]...)
	}

	aligned := make([]uint64, expectedLen)
	copy(aligned, targets)
	return aligned
}

func (s *Store) ApplyRewardToPlayerData(data *PlayerData, rewardID int, rewardCount int, now time.Time) error {
	switch rewardID {
	case 0:
		return addIntToInt32(&data.Cash, rewardCount, "cash")
	case 1:
		return addIntToInt32(&data.Asset, rewardCount, "asset")
	case 2:
		return s.AddEnergyToPlayerData(data, rewardCount, now)
	case 3:
		return addIntToInt32(&data.Shield, rewardCount, "shield")
	default:
		return fmt.Errorf("unsupported reward id %d", rewardID)
	}
}

func (s *Store) AddCashToPlayerData(data *PlayerData, delta int) error {
	value := int64(data.Cash) + int64(delta)
	if value < 0 {
		return ErrInsufficientCash
	}
	if value < -2147483648 || value > 2147483647 {
		return fmt.Errorf("cash overflow after applying delta %d", delta)
	}
	data.Cash = int32(value)
	return nil
}

func (s *Store) readEnergyParams() (int, int, int, error) {
	startEnergy, err := s.readRequiredParamInt(startEnergyParamKey)
	if err != nil {
		return 0, 0, 0, err
	}
	energyMax, err := s.readRequiredParamInt(energyMaxParamKey)
	if err != nil {
		return 0, 0, 0, err
	}
	recoverTime, err := s.readRequiredParamInt(energyRecoverTimeParamKey)
	if err != nil {
		return 0, 0, 0, err
	}
	return startEnergy, energyMax, recoverTime, nil
}

func (s *Store) refreshPlayerEnergy(data *PlayerData, now time.Time) (bool, error) {
	_, energyMax, recoverTime, err := s.readEnergyParams()
	if err != nil {
		return false, err
	}

	if data.Energy < 0 {
		data.Energy = 0
	}

	maxEnergy := int32(energyMax)
	changed := false
	if data.Energy >= maxEnergy {
		if data.Energy != maxEnergy {
			data.Energy = maxEnergy
			changed = true
		}
		if !data.EnergyRecoverAt.IsZero() {
			data.EnergyRecoverAt = time.Time{}
			changed = true
		}
		return changed, nil
	}

	if recoverTime <= 0 {
		return changed, nil
	}

	recoverDuration := time.Duration(recoverTime) * time.Second
	if data.EnergyRecoverAt.IsZero() {
		data.EnergyRecoverAt = now.Add(recoverDuration)
		return true, nil
	}
	if now.Before(data.EnergyRecoverAt) {
		return changed, nil
	}

	recovered := 1 + int(now.Sub(data.EnergyRecoverAt)/recoverDuration)
	newEnergy := int(data.Energy) + recovered
	if newEnergy >= energyMax {
		data.Energy = maxEnergy
		data.EnergyRecoverAt = time.Time{}
		return true, nil
	}

	data.Energy = int32(newEnergy)
	data.EnergyRecoverAt = data.EnergyRecoverAt.Add(time.Duration(recovered) * recoverDuration)
	return true, nil
}

func (s *Store) AddEnergyToPlayerData(data *PlayerData, delta int, now time.Time) error {
	_, energyMax, recoverTime, err := s.readEnergyParams()
	if err != nil {
		return err
	}

	value := int64(data.Energy) + int64(delta)
	if value < 0 {
		return ErrInsufficientEnergy
	}

	maxEnergy := int64(energyMax)
	if value > maxEnergy {
		value = maxEnergy
	}
	data.Energy = int32(value)

	if data.Energy >= int32(energyMax) {
		data.EnergyRecoverAt = time.Time{}
		return nil
	}
	if recoverTime <= 0 {
		data.EnergyRecoverAt = time.Time{}
		return nil
	}
	if data.EnergyRecoverAt.IsZero() {
		data.EnergyRecoverAt = now.Add(time.Duration(recoverTime) * time.Second)
	}
	return nil
}

func addIntToInt32(target *int32, delta int, fieldName string) error {
	value := int64(*target) + int64(delta)
	if value < -2147483648 || value > 2147483647 {
		return fmt.Errorf("%s overflow after applying delta %d", fieldName, delta)
	}
	*target = int32(value)
	return nil
}
