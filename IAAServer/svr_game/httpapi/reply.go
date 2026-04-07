package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"common/errorcode"
	"svr_game/game/model"
)

type DefaultValReply struct {
	PlayerID uint64 `json:"player_id,omitempty"`
	DebugVal int64  `json:"debug_val,omitempty"`
	ErrMsg   uint16 `json:"err_msg"`
}

type PlayerDataReply struct {
	PlayerID             uint64          `json:"player_id,omitempty"`
	CreationTime         uint64          `json:"creation_time,omitempty"`
	Cash                 int32           `json:"cash,omitempty"`
	Asset                int32           `json:"asset,omitempty"`
	Energy               int32           `json:"energy,omitempty"`
	EnergyRecoverAt      uint64          `json:"energy_recover_at,omitempty"`
	Shield               int32           `json:"shield,omitempty"`
	EventHistory         []model.EventID `json:"event_history,omitempty"`
	EventTargetPlayerIDs []uint64        `json:"event_target_player_ids,omitempty"`
	ErrMsg               uint16          `json:"err_msg"`
}

type AssetChangeReply struct {
	Cash            int32  `json:"cash,omitempty"`
	Asset           int32  `json:"asset,omitempty"`
	Energy          int32  `json:"energy,omitempty"`
	EnergyRecoverAt uint64 `json:"energy_recover_at,omitempty"`
	Shield          int32  `json:"shield,omitempty"`
	ErrMsg          uint16 `json:"err_msg"`
}

type TriggerEventReply struct {
	Cash              int32           `json:"cash,omitempty"`
	Asset             int32           `json:"asset,omitempty"`
	Energy            int32           `json:"energy,omitempty"`
	EnergyRecoverAt   uint64          `json:"energy_recover_at,omitempty"`
	Shield            int32           `json:"shield,omitempty"`
	EventHistoryDelta []model.EventID `json:"event_history_delta,omitempty"`
	TargetPlayerIDs   []uint64        `json:"target_player_ids,omitempty"`
	ErrMsg            uint16          `json:"err_msg"`
}

type RoomDataReply struct {
	CurrentRoomID   int     `json:"current_room_id"`
	FurnitureLevels []int32 `json:"furniture_levels"`
	ErrMsg          uint16  `json:"err_msg"`
}

type UpgradeFurnitureReply struct {
	CurrentRoomID   int     `json:"current_room_id"`
	FurnitureLevels []int32 `json:"furniture_levels"`
	Cash            int32   `json:"cash,omitempty"`
	Asset           int32   `json:"asset,omitempty"`
	ErrMsg          uint16  `json:"err_msg"`
}

func writeJSON(w http.ResponseWriter, status int, resp any) {
	setCommonHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func newDefaultValReply(data model.PlayerData) DefaultValReply {
	return DefaultValReply{
		PlayerID: data.PlayerID,
		DebugVal: data.DebugVal,
	}
}

func writeDefaultValJSON(w http.ResponseWriter, status int, reply DefaultValReply) {
	writeJSON(w, status, reply)
}

func writeDefaultValError(w http.ResponseWriter, status int, code errorcode.Code) {
	writeDefaultValJSON(w, status, DefaultValReply{ErrMsg: uint16(code)})
}

func newPlayerDataReply(data model.PlayerData) PlayerDataReply {
	return PlayerDataReply{
		PlayerID:             data.PlayerID,
		CreationTime:         uint64(data.CreatedAt.UTC().Unix()),
		Cash:                 data.Cash,
		Asset:                data.Asset,
		Energy:               data.Energy,
		EnergyRecoverAt:      unixTimeOrZero(data.EnergyRecoverAt),
		Shield:               data.Shield,
		EventHistory:         append([]model.EventID(nil), data.EventHistory...),
		EventTargetPlayerIDs: append([]uint64(nil), data.EventTargetPlayerIDs...),
	}
}

func writePlayerDataJSON(w http.ResponseWriter, status int, reply PlayerDataReply) {
	writeJSON(w, status, reply)
}

func writePlayerDataError(w http.ResponseWriter, status int, code errorcode.Code) {
	writePlayerDataJSON(w, status, PlayerDataReply{ErrMsg: uint16(code)})
}

func newAssetChangeReply(data model.PlayerData) AssetChangeReply {
	return AssetChangeReply{
		Cash:            data.Cash,
		Asset:           data.Asset,
		Energy:          data.Energy,
		EnergyRecoverAt: unixTimeOrZero(data.EnergyRecoverAt),
		Shield:          data.Shield,
	}
}

func writeAssetChangeJSON(w http.ResponseWriter, status int, reply AssetChangeReply) {
	writeJSON(w, status, reply)
}

func writeAssetChangeError(w http.ResponseWriter, status int, code errorcode.Code) {
	writeAssetChangeJSON(w, status, AssetChangeReply{ErrMsg: uint16(code)})
}

func newTriggerEventReply(historyDelta []model.EventID, targetPlayerIDs []uint64, data model.PlayerData) TriggerEventReply {
	return TriggerEventReply{
		Cash:              data.Cash,
		Asset:             data.Asset,
		Energy:            data.Energy,
		EnergyRecoverAt:   unixTimeOrZero(data.EnergyRecoverAt),
		Shield:            data.Shield,
		EventHistoryDelta: append([]model.EventID(nil), historyDelta...),
		TargetPlayerIDs:   append([]uint64(nil), targetPlayerIDs...),
	}
}

func writeTriggerEventJSON(w http.ResponseWriter, status int, reply TriggerEventReply) {
	writeJSON(w, status, reply)
}

func writeTriggerEventError(w http.ResponseWriter, status int, code errorcode.Code) {
	writeTriggerEventJSON(w, status, TriggerEventReply{ErrMsg: uint16(code)})
}

func newRoomDataReply(snapshot model.RoomSnapshot) RoomDataReply {
	return RoomDataReply{
		CurrentRoomID:   snapshot.CurrentRoomID,
		FurnitureLevels: append([]int32(nil), snapshot.FurnitureLevels...),
	}
}

func writeRoomDataJSON(w http.ResponseWriter, status int, reply RoomDataReply) {
	writeJSON(w, status, reply)
}

func writeRoomDataError(w http.ResponseWriter, status int, code errorcode.Code) {
	writeRoomDataJSON(w, status, RoomDataReply{ErrMsg: uint16(code)})
}

func newUpgradeFurnitureReply(snapshot model.RoomSnapshot, data model.PlayerData) UpgradeFurnitureReply {
	return UpgradeFurnitureReply{
		CurrentRoomID:   snapshot.CurrentRoomID,
		FurnitureLevels: append([]int32(nil), snapshot.FurnitureLevels...),
		Cash:            data.Cash,
		Asset:           data.Asset,
	}
}

func writeUpgradeFurnitureJSON(w http.ResponseWriter, status int, reply UpgradeFurnitureReply) {
	writeJSON(w, status, reply)
}

func writeUpgradeFurnitureError(w http.ResponseWriter, status int, code errorcode.Code) {
	writeUpgradeFurnitureJSON(w, status, UpgradeFurnitureReply{ErrMsg: uint16(code)})
}

func unixTimeOrZero(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UTC().Unix())
}
