import { act, render } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { useSummaryRefresh } from '../pages/equipment-summary-refresh';
import type { Props } from '../pages/equipment-detail';
import { pppoeAPI } from '../../platform/wails/presentation';
import type { PPResult } from '../../platform/wails/presentation';
vi.mock('../../platform/wails/presentation', () => ({
  pppoeAPI: { current: vi.fn() },
}));
function Probe({ p }: { p: Props }) {
  useSummaryRefresh(p, 'Resumo', false);
  return <div />;
}
const result: PPResult = {
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0 },
};
function fixture() {
  return {
    profile: 'company',
    activation: 'active',
    ready: true,
    connected: true,
    vault: true,
    locked: false,
    busy: false,
    selected: {
      id: 'synthetic',
      active: true,
      role: 'concentrator',
      credential_id: 'synthetic',
      fingerprint: 'pinned',
      result: { observed_at: new Date(Date.now() - 61000).toISOString() },
    },
    startOperation: vi.fn(async () => {}),
  } as unknown as Props;
}
it('does not start a device query after the opening unmounts during its collector check', async () => {
  let resolve!: (r: PPResult) => void;
  vi.mocked(pppoeAPI.current).mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  const p = fixture(),
    view = render(<Probe p={p} />);
  view.unmount();
  await act(async () => resolve(result));
  expect(p.startOperation).not.toHaveBeenCalled();
});
it('still refreshes a stale summary when its opening remains mounted', async () => {
  vi.mocked(pppoeAPI.current).mockResolvedValue(result);
  const p = fixture(),
    view = render(<Probe p={p} />);
  await act(async () => {
    await Promise.resolve();
  });
  expect(p.startOperation).toHaveBeenCalledOnce();
  view.unmount();
});
