import {
  ListEquipment,
  ListEquipmentFiltered,
  BulkUpdateEquipment,
} from '../wailsjs/go/presentation/App';
import type { equipment, presentation } from '../wailsjs/go/models';

export type InventoryFilters = {
  role: string;
  block: string;
  block_empty: boolean;
  model: string;
};
export type InventoryBulk = {
  items: { id: string; revision: number }[];
  block?: string;
  credential_id?: string;
};

export function listInventory(
  profile: string,
  activation: string,
  search: string,
  cursor: number,
  inactive: boolean,
  filters?: InventoryFilters,
) {
  if (!filters)
    return ListEquipment(profile, activation, search, cursor, inactive);
  const input = {
    search,
    cursor,
    inactive,
    role: filters?.role || '',
    block: filters?.block || '',
    block_empty: filters?.block_empty || false,
    model: filters?.model || '',
  };
  return ListEquipmentFiltered(
    profile,
    activation,
    input as presentation.InventoryFilters,
  );
}

export function bulkInventory(
  profile: string,
  activation: string,
  operation: string,
  input: InventoryBulk,
) {
  return BulkUpdateEquipment(
    profile,
    activation,
    operation,
    input as equipment.BulkUpdate,
  );
}
