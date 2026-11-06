import { act, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { pppoeReadAPI } from './presentation';
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
    <output aria-label="pontos">
      {JSON.stringify({ x: p.x, values: p.series[0].values })}
    </output>
  ),
}));
const device = {
  id: 'router',
  name: 'Sintético',
  revision: 1,
  active: true,
} as Device;
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function reading(started: string, ended: string, value: number) {
  return {
    scope: 'traffic',
    started_at: started,
    ended_at: ended,
    duration_ns: 2e9,
    rows: [
      {
        kind: 'session',
        row_id: 'row',
        login: 'synthetic',
        uptime: { known: true, min: 1, max: 1 },
        traffic: {
          points: [{ t: 1000, d: value, u: 10 }],
          download_bytes: '0',
          upload_bytes: '0',
          covered_ns: 1e9,
          gaps: 0,
        },
      },
    ],
  };
}
afterEach(() => {
  vi.useRealTimers();
  vi.resetAllMocks();
});
it('renews successful windows without erasing points and marks their interval as a gap', async () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-11-06T10:00:00Z'));
  vi.mocked(pppoeReadAPI.start).mockImplementation(async () =>
    result({
      job: {
        id: `job-${vi.mocked(pppoeReadAPI.start).mock.calls.length}`,
        state: 'RUNNING',
        code: 'OK',
      },
    }),
  );
  vi.mocked(pppoeReadAPI.status).mockImplementation(async (_p, _a, id) =>
    result({
      job: { id, state: id === 'job-1' ? 'SUCCEEDED' : 'RUNNING', code: 'OK' },
      read:
        id === 'job-1'
          ? reading('2026-11-06T10:00:00Z', '2026-11-06T10:00:02Z', 100)
          : reading('2026-11-06T10:00:55Z', '2026-11-06T10:00:57Z', 200),
    }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    expect(
      JSON.parse(screen.getByLabelText('pontos').textContent || '{}').values,
    ).toEqual([100]);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(54000);
    });
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(1001); });
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2);
    const graph = JSON.parse(
      screen.getByLabelText('pontos').textContent || '{}',
    );
    expect(graph.values).toEqual([100, null, 200]);
    expect(graph.x[0]).toBeLessThan(graph.x[1]);
    expect(graph.x[1]).toBeLessThan(graph.x[2]);
  } finally {
    view.unmount();
  }
});

it('stops after BUSY without an automatic retry', async () => {
  vi.useFakeTimers();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue({
    ...result(),
    code: 'BUSY',
  });
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
  } finally {
    view.unmount();
  }
});

it('stopping between windows prevents renewal and preserves the graph', async () => {
  vi.useFakeTimers();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    result({ job: { id: 'job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    result({
      job: { id: 'job', state: 'SUCCEEDED', code: 'OK' },
      read: reading('2026-11-06T10:00:00Z', '2026-11-06T10:00:02Z', 100),
    }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    screen.getByRole('button', { name: 'Parar' }).click();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    expect(
      JSON.parse(screen.getByLabelText('pontos').textContent || '{}').values,
    ).toEqual([100]);
    expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
  } finally {
    view.unmount();
  }
});

it('cancels an active traffic job when status fails and does not renew', async () => {
  vi.useFakeTimers();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    result({ job: { id: 'lost', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.status).mockRejectedValue(
    new Error('synthetic status failure'),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'lost',
    );
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('alert')).toBeVisible();
  } finally {
    view.unmount();
  }
});
