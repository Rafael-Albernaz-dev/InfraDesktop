package equipment

// RadioFamily is the driver identity proved by a reading, never a caller-selected guess.
type RadioFamily string

// Role identifies the operational capability of an inventory device.
type Role string

const (
	RoleAP                  Role        = "ap"
	RoleCPE                 Role        = "cpe"
	RolePTP                 Role        = "ptp"
	RoleConcentrator        Role        = "concentrator"
	RadioAuto               RadioFamily = "auto"
	RadioStandalone         RadioFamily = "auto-avulso"
	RadioUnsupported        RadioFamily = "unsupported"
	RadioRouterOSWirelessV6 RadioFamily = "routeros-wireless-v6"
	RadioAirOSV6            RadioFamily = "airos-v6"
	RadioIntelbrasWOM       RadioFamily = "intelbras-wom"
)

// RadioMode selects the bounded radio use case.
type RadioMode string

const (
	RadioOnce      RadioMode = "once"
	RadioMeasure   RadioMode = "measure"
	RadioLive      RadioMode = "live"
	RadioModeIndex RadioMode = "index"
)

// Transport is the inventory connection protocol, independent of its adapter.
type Transport string

const TransportSSH Transport = "ssh"
