package model

import "time"

type PlayerRoomData struct {
	PlayerID        uint64    `bson:"player_id,omitempty"`
	CurrentRoomID   int       `bson:"current_room_id"`
	FurnitureLevels []int32   `bson:"furniture_levels,omitempty"`
	CreatedAt       time.Time `bson:"created_at,omitempty"`
	UpdatedAt       time.Time `bson:"updated_at,omitempty"`
}

type RoomSnapshot struct {
	CurrentRoomID   int
	FurnitureLevels []int32
}
