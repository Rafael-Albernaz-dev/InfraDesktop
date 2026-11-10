// Wire values owned by equipment, represented at the UI contract boundary.
// These constants carry protocol values; driver admission remains in the local service.
export const RadioFamilies = {
  automatic: 'auto',
  standalone: 'auto-avulso',
  unsupported: 'unsupported',
  routerOSWirelessV6: 'routeros-wireless-v6',
  airOSV6: 'airos-v6',
  intelbrasWOM: 'intelbras-wom',
} as const;
export const RadioRoles = { ap: 'ap', cpe: 'cpe', ptp: 'ptp' } as const;
export const RadioModes = {
  once: 'once',
  measure: 'measure',
  live: 'live',
  index: 'index',
} as const;
export const RadioTransports = { ssh: 'ssh' } as const;
export const RadioCodes = {
  REMOTE_AUTH_FAILED: 'REMOTE_AUTH_FAILED',
  UNSUPPORTED: 'UNSUPPORTED',
} as const;
