package equipment

import "time"

// DropOfflineSeenElsewhere removes absences disproved by a later valid peer reading.
func DropOfflineSeenElsewhere(o *PPObservation, seen map[string]string) int {
	removed := 0
	kept := o.Rows[:0]
	for _, row := range o.Rows {
		if offlineSeenElsewhere(row, seen) {
			removed++
			continue
		}
		kept = append(kept, row)
	}
	o.Rows = kept
	o.Offline -= removed
	o.Count -= removed
	return removed
}

func offlineSeenElsewhere(row PPRow, seen map[string]string) bool {
	if row.State != "OFFLINE" || row.Login == "" {
		return false
	}
	absent, err := time.Parse(time.RFC3339Nano, row.AbsentStart)
	if err != nil {
		return false
	}
	ended, err := time.Parse(time.RFC3339Nano, seen[row.Login])
	return err == nil && !ended.Before(absent)
}
