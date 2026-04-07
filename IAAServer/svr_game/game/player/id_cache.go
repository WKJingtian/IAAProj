package player

func (s *Store) rememberPlayerID(openid string, playerID uint64) {
	if openid == "" || playerID == 0 {
		return
	}
	s.playerIDsByOpenID.Store(openid, playerID)
}

func (s *Store) cachedPlayerID(openid string) (uint64, bool) {
	if openid == "" {
		return 0, false
	}

	raw, ok := s.playerIDsByOpenID.Load(openid)
	if !ok {
		return 0, false
	}

	playerID, ok := raw.(uint64)
	if !ok || playerID == 0 {
		return 0, false
	}
	return playerID, true
}
