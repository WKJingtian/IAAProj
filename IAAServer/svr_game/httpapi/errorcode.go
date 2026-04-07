package httpapi

import (
	"errors"

	"common/errorcode"
	gameplayer "svr_game/game/player"
	"svr_game/game/room"
)

func triggerEventErrorCode(err error) errorcode.Code {
	switch {
	case gameplayer.IsInsufficientEnergy(err):
		return errorcode.TriggerEventInsufficientEnergy
	default:
		return errorcode.InternalError
	}
}

func upgradeFurnitureErrorCode(err error) errorcode.Code {
	switch {
	case errors.Is(err, room.ErrFurnitureNotFound):
		return errorcode.UpgradeFurnitureFurnitureNotFound
	case errors.Is(err, room.ErrFurnitureNotInCurrentRoom):
		return errorcode.UpgradeFurnitureFurnitureNotInCurrentRoom
	case errors.Is(err, room.ErrFurnitureMaxLevel):
		return errorcode.UpgradeFurnitureFurnitureMaxLevel
	case gameplayer.IsInsufficientCash(err):
		return errorcode.UpgradeFurnitureInsufficientCash
	default:
		return errorcode.InternalError
	}
}
