import {
  ChooseInventoryImport,
  ConfirmInventoryImport,
  ReviewInventoryImport,
  SaveInventoryTemplate,
} from '../../../wailsjs/go/presentation/App';
import type {
  EquipmentResult,
  ImportRow,
} from '../../features/inventory/inventory-import-types';
export type {
  EquipmentResult,
  ImportRow,
  ImportItem,
} from '../../features/inventory/inventory-import-types';
export const inventoryImportAPI = {
  choose: (p: string, a: string, b: string, c: string) =>
    ChooseInventoryImport(p, a, b, c) as Promise<
      EquipmentResult & { filename?: string }
    >,
  review: (p: string, a: string, rows: ImportRow[]) =>
    ReviewInventoryImport(p, a, rows) as Promise<EquipmentResult>,
  confirm: (p: string, a: string, op: string, rows: ImportRow[]) =>
    ConfirmInventoryImport(p, a, op, rows) as Promise<EquipmentResult>,
  template: SaveInventoryTemplate,
};
