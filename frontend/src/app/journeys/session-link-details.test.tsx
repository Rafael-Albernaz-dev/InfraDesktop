import { renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { useSessionLinkDetails } from '../../features/radio/session-link-details';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import type { RadioLinkState } from '../../features/radio/radio-link-state';
vi.mock('../../platform/wails/presentation', () => ({
  equipmentAPI: { get: vi.fn() },
  radioAPI: { current: vi.fn() },
}));
const state = {
  profile: 'company',
  activation: 'active',
  link: { ap_id: 'ap-fixture', cpe_id: 'cpe-fixture' },
  radio: { view: {} },
} as unknown as RadioLinkState;
const reply = (id: string) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { device: { id, name: `Equip ${id}` } },
});
it('uses only local readings and device names matching the requested scope and identities', async () => {
  vi.mocked(equipmentAPI.get).mockImplementation(
    async (_p, _a, id) => reply(id) as never,
  );
  vi.mocked(radioAPI.current).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: { reading: { device_id: 'cpe-fixture', compatible: true } },
  } as never);
  const { result } = renderHook(() => useSessionLinkDetails(state, true));
  await waitFor(() =>
    expect(result.current.reading?.device_id).toBe('cpe-fixture'),
  );
  expect(result.current.names['ap-fixture']).toBe('Equip ap-fixture');
  expect(radioAPI.current).toHaveBeenCalledWith(
    'company',
    'active',
    'cpe-fixture',
  );
});
it('does not display a reading returned for another activation', async () => {
  vi.mocked(equipmentAPI.get).mockImplementation(
    async (_p, _a, id) => reply(id) as never,
  );
  vi.mocked(radioAPI.current).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'other',
    data: { reading: { device_id: 'cpe-fixture', compatible: true } },
  } as never);
  const { result } = renderHook(() => useSessionLinkDetails(state, true));
  await waitFor(() =>
    expect(result.current.names['cpe-fixture']).toBe('Equip cpe-fixture'),
  );
  expect(result.current.reading).toBeUndefined();
});
