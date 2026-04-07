package event

import "svr_game/game/model"

func findEventChainIndex(chains []EventChainState, rootEventID EventID) int {
	for index, chain := range chains {
		if chain.RootEventID == rootEventID {
			return index
		}
	}
	return -1
}

func removePendingEvent(chains []EventChainState, rootEventID EventID, eventID EventID) []EventChainState {
	index := findEventChainIndex(chains, rootEventID)
	if index < 0 {
		return chains
	}

	delete(chains[index].PendingPool, model.FormatEventID(eventID))
	if len(chains[index].PendingPool) == 0 {
		chains[index].PendingPool = nil
	}
	return chains
}

func mergePendingPool(chains []EventChainState, rootEventID EventID, pool map[string]int32) []EventChainState {
	if len(pool) == 0 {
		return chains
	}

	index := findEventChainIndex(chains, rootEventID)
	if index < 0 {
		return append(chains, EventChainState{
			RootEventID: rootEventID,
			PendingPool: clonePendingPool(pool),
		})
	}

	if chains[index].PendingPool == nil {
		chains[index].PendingPool = make(map[string]int32, len(pool))
	}
	for key, weight := range pool {
		chains[index].PendingPool[key] = weight
	}
	return chains
}

func removeEventChain(chains []EventChainState, rootEventID EventID) []EventChainState {
	index := findEventChainIndex(chains, rootEventID)
	if index < 0 {
		return chains
	}

	last := len(chains) - 1
	chains[index] = chains[last]
	return chains[:last]
}

func removeEventChainIfEmpty(chains []EventChainState, rootEventID EventID) []EventChainState {
	index := findEventChainIndex(chains, rootEventID)
	if index < 0 {
		return chains
	}
	if len(chains[index].PendingPool) > 0 {
		return chains
	}
	return removeEventChain(chains, rootEventID)
}

func hasEventChain(chains []EventChainState, rootEventID EventID) bool {
	return findEventChainIndex(chains, rootEventID) >= 0
}

func clonePendingPool(pool map[string]int32) map[string]int32 {
	if len(pool) == 0 {
		return nil
	}

	cloned := make(map[string]int32, len(pool))
	for key, weight := range pool {
		cloned[key] = weight
	}
	return cloned
}
