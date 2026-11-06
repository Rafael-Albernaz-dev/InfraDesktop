import type { Device } from './equipment-api';
import type { PPRow, PPObservation } from './presentation';
export type PPPoESessionDetailProps = {
  selected: PPRow;
  current: PPObservation;
  detailHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  detailTab: 'traffic' | 'information' | 'link' | 'history';
  setDetailTab: (t: 'traffic' | 'information' | 'link' | 'history') => void;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  detailIntentRef: React.MutableRefObject<number>;
  select: (r: PPRow | undefined) => void;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  openEquipment?: (id: string) => void;
  back: () => void;
};
