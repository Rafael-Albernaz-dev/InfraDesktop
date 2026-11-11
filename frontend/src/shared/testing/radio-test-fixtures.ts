import {
  RadioFamilies,
  RadioRoles,
} from '../../features/equipment/radio-contract';
import type {
  RadioReading,
  RadioResult,
  RadioView,
} from '../../features/equipment/radio-api';
import type { Device } from '../../features/equipment/equipment-api';

export const fixtureAPID = 'synthetic-ap';
export const fixtureCPEID = 'synthetic-cpe';
export const device = {
  id: fixtureAPID,
  revision: 3,
  role: RadioRoles.ap,
  active: true,
  fingerprint: 'pinned',
} as Device;
export const response = (data?: Partial<RadioView>): RadioResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, more: false, next_offset: 0, ...data },
});
export const reading: RadioReading = {
  id: 'read',
  device_id: fixtureAPID,
  family: RadioFamilies.routerOSWirelessV6,
  version: '6.49.10',
  perspective: 'AP',
  observed_at: '2026-10-29T00:00:00Z',
  compatible: true,
  revision: 3,
  credential_revision: 1,
  connection: 'pinned',
  interfaces: [],
  clients: [
    {
      mac: '02:00:00:00:10:03',
      name: 'CPE-I1',
      interface: 'wlan1',
      bssid: '02:00:00:00:10:05',
      wireless_uptime: '1h',
      last_activity: '90ms',
      metrics: [
        {
          name: 'signal',
          value: -70,
          unit: 'dBm',
          direction: 'CPE → AP',
          perspective: 'AP',
          device_id: fixtureAPID,
          interface: 'wlan1',
          observed_at: '2026-10-29T00:00:00Z',
          quality: 'observed',
        },
      ],
    },
  ],
  metrics: [],
  seconds: 1,
  had_clients: true,
};
export const link = {
  cpe_id: fixtureCPEID,
  ap_id: fixtureAPID,
  mac: '02:00:00:00:10:03',
  interface: 'wlan1',
  bssid: '02:00:00:00:10:05',
  evidence: 'MAC na registration table',
  origin: 'sugerido-confirmado',
  concentrator_id: 'concentrator',
  scope: 'pppoe-active-v1',
  login: 'X',
  confirmed_at: '2026-10-29T00:00:00Z',
  confirmed_now: true,
};
