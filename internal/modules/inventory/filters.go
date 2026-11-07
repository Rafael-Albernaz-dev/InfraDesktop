package inventory

import (
	"sort"
	"strings"

	"infradesktop/internal/modules/equipment"
)

func validateInventoryFilters(q Command) Code {
	if !validPulseFilter(q.State) {
		return Invalid
	}
	if q.Role != "" && !equipment.ValidInventoryRole(q.Role) ||
		!equipment.ValidBlock(q.Block) || q.BlockEmpty && q.Block != "" || len(q.Model) > 160 {
		return Invalid
	}
	return OK
}

func inventoryModel(d Device) string {
	if d.Result != nil && d.Result.Board != "" {
		return d.Result.Board
	}
	if d.RadioSummary != nil {
		return d.RadioSummary.Model
	}
	return ""
}

func inventoryOptions(devices []Device) ([]string, []string) {
	blocks, models := map[string]bool{}, map[string]bool{}
	for _, d := range devices {
		if d.Block != "" {
			blocks[d.Block] = true
		}
		if m := inventoryModel(d); m != "" {
			models[m] = true
		}
	}
	return sortedOptions(blocks), sortedOptions(models)
}

func sortedOptions(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func filterDevices(devices []Device, q Command) []Device {
	filtered := []Device{}
	needle := strings.ToLower(q.Search)
	for _, d := range devices {
		if !inventoryMatches(d, q, needle) || !pulseMatches(d, q.State) {
			continue
		}
		if d.Result != nil {
			summary := *d.Result
			summary.Interfaces = nil
			d.Result = &summary
		}
		filtered = append(filtered, d)
	}
	return filtered
}

func inventoryMatches(d Device, q Command, needle string) bool {
	if q.Inactive && d.Active || !q.Inactive && !q.IncludeInactive && !d.Active {
		return false
	}
	if q.Role != "" && q.Role != d.Role || q.BlockEmpty && d.Block != "" ||
		q.Block != "" && q.Block != d.Block || q.Model != "" && q.Model != inventoryModel(d) {
		return false
	}
	return strings.Contains(strings.ToLower(d.Name+" "+d.Address), needle)
}
