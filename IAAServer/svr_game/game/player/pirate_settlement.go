package player

const (
	pirateBlockedEventIDParamKey = "pirate_blocked_event_id"
	pirateHitEventIDParamKey     = "pirate_hit_event_id"
	pirateHitCashLossParamKey    = "pirate_hit_cash_loss_percent"
)

func (s *Store) settlePirateIncoming(data *PlayerData) (bool, error) {
	if len(data.PirateIncomingFrom) == 0 {
		return false, nil
	}

	maxHistory, err := s.readRequiredParamInt(maxEventHistoryParamKey)
	if err != nil {
		return false, err
	}
	blockedEventID, err := s.readRequiredEventIDParam(pirateBlockedEventIDParamKey)
	if err != nil {
		return false, err
	}
	hitEventID, err := s.readRequiredEventIDParam(pirateHitEventIDParamKey)
	if err != nil {
		return false, err
	}
	hitCashLossPercent, err := s.readRequiredParamFloat(pirateHitCashLossParamKey)
	if err != nil {
		return false, err
	}

	for _, sourcePlayerID := range data.PirateIncomingFrom {
		if data.Shield > 0 {
			data.Shield--
			appendEventHistoryEntry(data, blockedEventID, sourcePlayerID, maxHistory)
			continue
		}

		cashLoss := int64(float64(data.Cash) * hitCashLossPercent)
		remainingCash := int64(data.Cash) - cashLoss
		if remainingCash < 0 {
			remainingCash = 0
		}
		data.Cash = int32(remainingCash)
		appendEventHistoryEntry(data, hitEventID, sourcePlayerID, maxHistory)
	}

	data.PirateIncomingFrom = nil
	return true, nil
}
