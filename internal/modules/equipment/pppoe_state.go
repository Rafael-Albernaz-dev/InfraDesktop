package equipment

func PPValidState(state string) bool { return state == "" || state == "ONLINE" || state == "OFFLINE" }

func PPValidSort(sort string) bool {
	switch sort {
	case "", "login", "address", "uptime", "row", "state":
		return true
	}
	return false
}

// Filtering deltas preserves the complete source comparison, including retained
// OFFLINE records and ambiguity. A base-only delta keeps its recorded state.
func PPDeltaMatchesQuery(delta PPDelta, query *PPQuery) bool {
	if query == nil {
		return true
	}
	if query.Category != "" && query.Category != delta.Category {
		return false
	}
	if query.State == "" {
		return true
	}
	session := delta.Target
	if session == nil {
		session = delta.Base
	}
	if session == nil {
		return false
	}
	state := session.State
	if state == "" {
		state = "ONLINE"
	}
	return state == query.State
}
