import { RadioTransports } from './radio-contract';
import type { Device, EquipmentResult } from './equipment-api';
export const device = {
  id: 'device-X',
  role: 'concentrator',
  name: 'Router X',
  address: '127.0.0.1',
  port: 22,
  transport: RadioTransports.ssh,
  credential_id: 'credential',
  revision: 1,
  active: true,
  fingerprint: '',
  trust_origin: '',
} as Device;
export const data = (devices: Device[] = [device]) =>
  ({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { devices, credentials: [], next_cursor: 0, vault_available: true },
  }) as EquipmentResult;
