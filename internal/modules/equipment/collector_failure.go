package equipment

// CollectorDefect identifies an incompatible InfraDesktop collector, not absent
// device capability. Reasons are closed structural enums, never remote content.
const CollectorDefect Code = "COLLECTOR_DEFECT"

func ValidCollectorReason(code Code, reason string) bool {
	if code == CollectorDefect {
		return reason == "O:syntax" || reason == "A:syntax" || reason == "READ:syntax"
	}
	if code != Partial {
		return false
	}
	switch reason {
	case "READ:inconsistent", "O:execution", "A:execution", "O:header", "Z:trailing", "Z:fields", "Z:footer", "Z:missing", "wire:line-limit", "wire:codec", "ROW:fields", "ROW:duplicate", "ROW:type", "R:fields", "R:echo-mismatch", "R:property", "C:fields", "G:fields", "G:group", "S:fields", "S:service", "K:fields", "I:fields", "P:fields", "P:echo-mismatch", "A:header", "A:fields", "A:footer", "A:missing", "F:identity", "F:time", "F:fields", "G:auth":
		return true
	}
	return false
}
