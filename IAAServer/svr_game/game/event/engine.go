package event

import (
	"math/rand"
	"time"

	"svr_game/game/model"
	"svr_game/staticdata"
)

type EventID = model.EventID
type EventChainState = model.EventChainState
type PlayerData = model.PlayerData
type EventRow = staticdata.EventRow

type MutationOps struct {
	AddEnergy           func(*PlayerData, int, time.Time) error
	ApplyReward         func(*PlayerData, int, int, time.Time) error
	ApplyPirateIncoming func(uint64) (uint64, bool, error)
}

type TriggerResult struct {
	EventHistoryDelta []EventID
	TargetPlayerIDs   []uint64
}

type Engine struct {
	staticData  *staticdata.StaticData
	randIntn    func(int) int
	cheatModeOn int
}

func NewEngine(staticData *staticdata.StaticData, randIntn func(int) int, cheatModeOn int) *Engine {
	if randIntn == nil {
		randIntn = rand.Intn
	}

	return &Engine{
		staticData:  staticData,
		randIntn:    randIntn,
		cheatModeOn: cheatModeOn,
	}
}
