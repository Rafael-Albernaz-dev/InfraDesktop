import type { Device } from '../equipment/equipment-api';
import type { LinkedSessionTarget } from '../pppoe/linked-session-navigation';
export type RadioEquipmentProps = {
  profile: string;
  activation: string;
  device: Device;
  blocked: boolean;
  devices?: Device[];
  openEquipment?: (id: string) => void;
  openSession?: (target: LinkedSessionTarget) => void;
};
