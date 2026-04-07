package room

import "errors"

var (
	ErrFurnitureNotFound         = errors.New("furniture not found")
	ErrFurnitureNotInCurrentRoom = errors.New("furniture does not belong to current room")
	ErrFurnitureMaxLevel         = errors.New("furniture is already at max level")
)

func IsBadRequest(err error) bool {
	return errors.Is(err, ErrFurnitureNotFound) ||
		errors.Is(err, ErrFurnitureNotInCurrentRoom) ||
		errors.Is(err, ErrFurnitureMaxLevel)
}
