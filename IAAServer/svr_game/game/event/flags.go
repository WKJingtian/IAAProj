package event

import "fmt"

const flagPirateIncoming = "pirate_incoming"

func applySpecialFlags(playerData *PlayerData, eventRow EventRow, ops MutationOps) (uint64, error) {
	if len(eventRow.Flags) == 0 {
		return 0, nil
	}

	var targetPlayerID uint64
	for _, flag := range eventRow.Flags {
		switch flag {
		case "", "[]":
			continue
		case flagPirateIncoming:
			if ops.ApplyPirateIncoming == nil {
				return 0, fmt.Errorf("event %d pirate_incoming is not configured", eventRow.ID)
			}
			hitTargetPlayerID, hit, err := ops.ApplyPirateIncoming(playerData.PlayerID)
			if err != nil {
				return 0, err
			}
			if hit {
				if targetPlayerID != 0 && targetPlayerID != hitTargetPlayerID {
					return 0, fmt.Errorf("event %d produced multiple target player ids", eventRow.ID)
				}
				targetPlayerID = hitTargetPlayerID
			}
		}
	}

	return targetPlayerID, nil
}
