import type { Device } from './equipment-api';
import type { useEquipmentState } from './equipment-state';

export type InventoryState = ReturnType<typeof useEquipmentState> & {
  selectedRows: Record<string, Device>;
  setSelectedRows: React.Dispatch<React.SetStateAction<Record<string, Device>>>;
};
