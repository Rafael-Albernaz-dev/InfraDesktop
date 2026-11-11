import { expect, it, vi } from 'vitest';
import { equipmentAPI } from '../../features/equipment/equipment-api';
import { createRecoverySave } from '../../features/radio/radio-credential-state';
import { device } from '../../shared/testing/radio-test-fixtures';

it('preserves the block when changing the saved radio credential', async () => {
  const target = { ...device, block: 'Torre Norte', credential_id: 'old' };
  const generation = {};
  const save = vi.spyOn(equipmentAPI, 'save').mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: { device: target },
  } as Awaited<ReturnType<typeof equipmentAPI.save>>);
  const state = {
    profile: 'company',
    activation: 'active',
    target,
    busy: false,
    recoveryLife: { current: generation },
    alive: { current: true },
    setBusy: vi.fn(),
    setTarget: vi.fn(),
    setMessage: vi.fn(),
    setOpen: vi.fn(),
    changed: vi.fn(),
  } as unknown as Parameters<typeof createRecoverySave>[0];
  await createRecoverySave(state)('new');
  expect(save).toHaveBeenCalledWith(
    'company',
    'active',
    expect.objectContaining({
      block: 'Torre Norte',
      credential_id: 'new',
    }),
  );
  save.mockRestore();
});
