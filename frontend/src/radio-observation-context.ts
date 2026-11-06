import type { Device } from './equipment-api';
export type RadioObservationContext = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
};
