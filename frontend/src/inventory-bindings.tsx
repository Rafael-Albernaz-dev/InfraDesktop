import type { EquipmentResult } from './presentation';

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
type FilterInput = InventoryFilters & {
  search: string;
  cursor: number;
  inactive: boolean;
};
export function createInventoryBindings(
  list: (profile: string, activation: string, search: string, cursor: number, inactive: boolean) => Promise<EquipmentResult>,
  listFiltered: (profile: string, activation: string, input: FilterInput) => Promise<EquipmentResult>,
  bulk: (profile: string, activation: string, operation: string, input: InventoryBulk) => Promise<EquipmentResult>,
) {
  return {
    list: (profile: string, activation: string, search: string, cursor: number, inactive: boolean, filters?: InventoryFilters) => {
      if (!filters) return list(profile, activation, search, cursor, inactive);
      return listFiltered(profile, activation, { search, cursor, inactive, ...filters });
    },
    bulk,
  };
}
