package access

func applyInventoryColumns(current *State, q Command) Code {
	if q.Columns == nil || !ValidInventoryColumns(*q.Columns) {
		return Invalid
	}
	current.Account.Columns = append([]string(nil), (*q.Columns)...)
	return OK
}

// ValidInventoryColumns permits only known columns; the name column is fixed.
func ValidInventoryColumns(columns []string) bool {
	if len(columns) == 0 || len(columns) > 17 || columns[0] != "equipment" {
		return false
	}
	allowed := map[string]bool{
		"equipment": true, "status": true, "role": true, "block": true, "model": true,
		"address": true, "online": true, "offline": true, "uptime": true, "cpu": true,
		"last_reading": true, "memory": true, "temperature": true, "voltage": true,
		"system": true, "credential": true, "query": true,
	}
	seen := map[string]bool{}
	for _, column := range columns {
		if !allowed[column] || seen[column] {
			return false
		}
		seen[column] = true
	}
	return true
}
