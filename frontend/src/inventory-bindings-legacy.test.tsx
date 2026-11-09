import { expect, it, vi } from 'vitest';
import { createInventoryBindings } from './inventory-bindings';
import type { EquipmentResult } from './presentation';

it('keeps inactive-only calls for credential usage and includes inactive rows only when requested by the list', async () => {
  const result = {} as EquipmentResult;
  const legacy = vi.fn(async () => result);
  const filtered = vi.fn(async () => result);
  const { list } = createInventoryBindings(
    legacy,
    filtered,
    vi.fn(async () => result),
  );
  await list('company', 'active', '', 0, true);
  expect(legacy).toHaveBeenCalledWith('company', 'active', '', 0, true);
  await list('company', 'active', '', 0, true, {
    role: '',
    block: '',
    block_empty: false,
    model: '',
  });
  expect(filtered).toHaveBeenCalledWith(
    'company',
    'active',
    expect.objectContaining({ inactive: true }),
  );
});
