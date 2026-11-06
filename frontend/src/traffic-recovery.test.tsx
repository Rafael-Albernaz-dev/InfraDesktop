import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { pppoeReadAPI as api } from './presentation';
import type { Device } from './equipment-api';
import type { ChartProps } from './ui/Chart';
vi.mock('./presentation', () => ({
  pppoeReadAPI: {
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    stop: vi.fn(),
    page: vi.fn(),
  },
  pppoeDiagnosticAPI: {},
  deviceMonitoringAPI: {},
  pppoeAPI: {},
}));
vi.mock('./ui/Chart', () => ({
  Chart: (p: ChartProps) => (
    <output aria-label="pontos">{JSON.stringify(p.series[0].values)}</output>
  ),
}));
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function panel() {
  return render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={{ id: 'router', revision: 1 } as Device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
}
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.resetAllMocks();
});
it.each([
  'OFFLINE',
  'DEVICE_INACTIVE',
  'TRUST_REQUIRED',
  'CREDENTIAL_REQUIRED',
  'UNAUTHENTICATED',
])('does not retry a permanent %s rejection', async (code) => {
  vi.useFakeTimers();
  vi.mocked(api.start).mockResolvedValue({ ...result(), code });
  vi.mocked(api.cancelOperation).mockResolvedValue(result());
  const view = panel();
  await advance(1);
  await advance(120000);
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
  view.unmount();
});
it('bounds consecutive status failures and cancels each failed window before retrying', async () => {
  vi.useFakeTimers();
  vi.mocked(api.start).mockImplementation(async () =>
    result({
      job: {
        id: `job-${vi.mocked(api.start).mock.calls.length}`,
        state: 'RUNNING',
        code: 'OK',
      },
    }),
  );
  vi.mocked(api.status).mockRejectedValue(
    new Error('synthetic transport failure'),
  );
  vi.mocked(api.cancel).mockResolvedValue(result());
  const view = panel();
  await advance(1);
  for (const delay of [5000, 15000, 30000]) await advance(delay);
  await advance(120000);
  expect(api.start).toHaveBeenCalledTimes(4);
  expect(api.cancel).toHaveBeenCalledTimes(4);
  expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
  view.unmount();
});
it('retains the graph through a hidden interval and inserts a gap on resuming', async () => {
  vi.useFakeTimers();
  const doc = render(<div />).container.ownerDocument;
  let visibility: DocumentVisibilityState = 'visible';
  vi.spyOn(doc, 'visibilityState', 'get').mockImplementation(() => visibility);
  vi.mocked(api.start).mockImplementation(async () =>
    result({
      job: {
        id: `job-${vi.mocked(api.start).mock.calls.length}`,
        state: 'RUNNING',
        code: 'OK',
      },
    }),
  );
  vi.mocked(api.status).mockImplementation(async (_p, _a, id) =>
    result({
      job: { id, state: 'RUNNING', code: 'OK' },
      read: {
        scope: 'traffic',
        started_at:
          id === 'job-1' ? '2026-11-06T10:00:00Z' : '2026-11-06T10:01:00Z',
        ended_at: id,
        duration_ns: 2e9,
        rows: [
          {
            kind: 'session',
            row_id: 'row',
            login: 'synthetic',
            uptime: { known: true, min: 1, max: 1 },
            traffic: {
              points: [{ t: 1000, d: id === 'job-1' ? 100 : 200, u: 10 }],
              download_bytes: '0',
              upload_bytes: '0',
              covered_ns: 1e9,
              gaps: 0,
            },
          },
        ],
      },
    }),
  );
  vi.mocked(api.cancel).mockResolvedValue(result());
  const view = panel();
  await advance(1);
  expect(
    JSON.parse(screen.getByLabelText('pontos').textContent || '[]'),
  ).toEqual([100]);
  visibility = 'hidden';
  fireEvent(doc, new Event('visibilitychange'));
  await advance(60000);
  expect(
    JSON.parse(screen.getByLabelText('pontos').textContent || '[]'),
  ).toEqual([100]);
  visibility = 'visible';
  fireEvent(doc, new Event('visibilitychange'));
  await advance(1);
  expect(
    JSON.parse(screen.getByLabelText('pontos').textContent || '[]'),
  ).toEqual([100, null, 200]);
  view.unmount();
});
it('does not renew after unmounting while cancellation is pending', async () => {
  vi.useFakeTimers();
  vi.mocked(api.start).mockResolvedValue(
    result({ job: { id: 'job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockRejectedValue(new Error('synthetic'));
  let cancelled: ((r: ReturnType<typeof result>) => void) | undefined;
  vi.mocked(api.cancel).mockImplementation(
    () =>
      new Promise((resolve) => {
        cancelled = resolve;
      }),
  );
  const view = panel();
  await advance(1);
  view.unmount();
  await act(async () => {
    cancelled?.(result());
  });
  await advance(120000);
  expect(api.start).toHaveBeenCalledTimes(1);
});
