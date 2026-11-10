import type { Device } from '../equipment/equipment-api';
export type RadioObservationContext = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
};
