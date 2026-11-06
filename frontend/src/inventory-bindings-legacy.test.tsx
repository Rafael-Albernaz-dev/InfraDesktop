import { expect, it, vi } from 'vitest';
import { ListEquipment, ListEquipmentFiltered } from '../wailsjs/go/presentation/App';
import { listInventory } from './inventory-bindings';

vi.mock('../wailsjs/go/presentation/App', () => ({
  ListEquipment: vi.fn(async () => ({})),
  ListEquipmentFiltered: vi.fn(async () => ({})),
  BulkUpdateEquipment: vi.fn(),
}));

it('keeps inactive-only calls for credential usage and includes inactive rows only when requested by the list', async () => {
  await listInventory('company', 'active', '', 0, true);
  expect(ListEquipment).toHaveBeenCalledWith('company', 'active', '', 0, true);
  await listInventory('company', 'active', '', 0, true, {role:'', block:'', block_empty:false, model:''});
  expect(ListEquipmentFiltered).toHaveBeenCalledWith('company', 'active', expect.objectContaining({inactive:true}));
});
