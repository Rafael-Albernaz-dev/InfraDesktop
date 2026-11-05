package equipment

// RadioFamily is the driver identity proved by a reading, never a caller-selected guess.
type RadioFamily string

const (
	RadioAuto               RadioFamily = "auto"
	RadioStandalone         RadioFamily = "auto-avulso"
	RadioUnsupported        RadioFamily = "unsupported"
	RadioRouterOSWirelessV6 RadioFamily = "routeros-wireless-v6"
	RadioAirOSV6            RadioFamily = "airos-v6"
	RadioIntelbrasWOM       RadioFamily = "intelbras-wom"
)
