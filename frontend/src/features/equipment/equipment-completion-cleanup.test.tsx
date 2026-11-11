import { act, render, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useDetailDeviceSyncEffects } from './equipment-effects';
import { equipmentAPI as api } from './equipment-api';
import type { Device, Job, EquipmentResult } from './equipment-api';
vi.mock('./equipment-api', async () =>
  (await import('../../shared/testing/equipment-api-mock')).equipmentAPIMock({
    equipmentMessage: (code: string) => code,
  }),
);
function fixture() {
  return {
    profile: 'company',
    activation: 'active',
    connected: true,
    detailOpen: false,
    detailReadRef: { current: 0 },
    surfaceRef: { current: 0 },
    job: {
      id: 'job',
      device_id: 'synthetic',
      kind: 'device.query-start',
      state: 'SUCCEEDED',
      code: 'OK',
    } as Job,
    setSelected: vi.fn(),
    setDetailOpen: vi.fn(),
    setError: vi.fn(),
    onSelect: vi.fn(),
  };
}
function Probe({ p }: { p: ReturnType<typeof fixture> }) {
  useDetailDeviceSyncEffects(p);
  return <div />;
}
const device = { id: 'synthetic', revision: 1 } as Device;
const result: EquipmentResult = {
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: {
    device,
    devices: [],
    credentials: [],
    next_cursor: 0,
    vault_available: true,
  },
};
afterEach(() => vi.resetAllMocks());
it('does not dispatch a completion lookup when its owner unmounts before queued work', async () => {
  vi.mocked(api.get).mockResolvedValue(result);
  const p = fixture(),
    view = render(<Probe p={p} />);
  view.unmount();
  await act(async () => {
    await Promise.resolve();
  });
  expect(api.get).not.toHaveBeenCalled();
  expect(p.setSelected).not.toHaveBeenCalled();
});
it('handles a synchronously unavailable bridge while its owner is still mounted', async () => {
  vi.mocked(api.get).mockImplementation(() => {
    throw new Error('synthetic bridge unavailable');
  });
  const p = fixture();
  const view = render(<Probe p={p} />);
  await waitFor(() =>
    expect(p.setError).toHaveBeenCalledWith('SERVICE_UNAVAILABLE'),
  );
  expect(p.setSelected).not.toHaveBeenCalled();
  view.unmount();
});
it('still applies a valid completion lookup on its original surface', async () => {
  vi.mocked(api.get).mockResolvedValue(result);
  const p = fixture(),
    view = render(<Probe p={p} />);
  await waitFor(() => expect(p.setSelected).toHaveBeenCalledWith(device));
  expect(p.setDetailOpen).toHaveBeenCalledWith(true);
  expect(p.onSelect).toHaveBeenCalledWith('synthetic');
  expect(api.get).toHaveBeenCalledExactlyOnceWith(
    'company',
    'active',
    'synthetic',
  );
  view.unmount();
});
it('does not apply a completion response after unmount', async () => {
  let finish!: (r: EquipmentResult) => void;
  vi.mocked(api.get).mockReturnValue(
    new Promise<EquipmentResult>((resolve) => {
      finish = resolve;
    }),
  );
  const p = fixture(),
    view = render(<Probe p={p} />);
  await waitFor(() => expect(api.get).toHaveBeenCalledOnce());
  view.unmount();
  await act(async () => finish(result));
  expect(p.setSelected).not.toHaveBeenCalled();
  expect(p.onSelect).not.toHaveBeenCalled();
});
