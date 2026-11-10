import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { DeviceMonitoring } from '../../features/pppoe/device-monitoring';
import { PPPoEReadPanel } from '../../features/pppoe/pppoe-diagnostics';

import {
  pppoeDiagnosticAPI as api,
  deviceMonitoringAPI as monitoringAPI,
  pppoeReadAPI as readAPI,
  pppoeAPI as sessions,
} from '../../platform/wails/presentation';
import { pendingPPPoE } from '../../features/access/equipment-receipts';
import type { Device } from '../../features/equipment/equipment-api';
import type {
  PDDiagnostic,
  PDRow,
  PPResult,
} from '../../platform/wails/presentation';
vi.mock('../../platform/wails/presentation', () => ({
  deviceMonitoringAPI: { current: vi.fn() },
  pppoeReadAPI: {
    stop: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    page: vi.fn(),
  },
  pppoeDiagnosticAPI: {
    catalog: vi.fn(),
    radius: vi.fn(),
    ping: vi.fn(),
    current: vi.fn(),
    list: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
  },
  pppoeAPI: {
    groups: vi.fn(),
    current: vi.fn(),
    references: vi.fn(),
    list: vi.fn(),
    start: vi.fn(),
  },
}));
const device = {
  id: 'device',
  name: 'Concentrador sintético',
  revision: 2,
  active: true,
  fingerprint: 'pinned',
} as Device;
const response = (data: PPResult['data'] = { total: 0 }): PPResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data,
});
const meta = (scope = 'catalog'): PDDiagnostic => ({
  diagnostic_id: scope + '-snapshot',
  device_id: device.id,
  scope,
  count: 1,
  quality: { status: 'complete' },
  provenance: { version: '7.26', ended_at: '2026-10-26T00:00:00Z' },
});
const rows: PDRow[] = [
  {
    id: 'row',
    kind: 'service',
    name: 'Serviço de teste',
    parent: 'VLAN de teste',
    relation: 'resolved',
    disabled: false,
    range: '100-115',
    accept_untagged: true,
  },
];
function deferred() {
  let resolve!: (value: PPResult) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<PPResult>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(readAPI.stop).mockResolvedValue(response());
  vi.mocked(readAPI.cancelOperation).mockResolvedValue(response());
  vi.mocked(sessions.groups).mockResolvedValue(response());
  pendingPPPoE.clear();
  vi.mocked(api.current).mockImplementation(async (_p, _a, _d, scope) =>
    response({ diagnostic: meta(scope), total: 1 }),
  );
  vi.mocked(api.list).mockResolvedValue(
    response({ diagnostic_rows: rows, total: 1 }),
  );
  vi.mocked(sessions.current).mockResolvedValue(response());
  vi.mocked(sessions.references).mockResolvedValue(
    response({ references: [], total: 0 }),
  );
});
it('stored series preserves unknowns and package divergence across pages', async () => {
  const point = {
    at: '2026-10-28T01:00:00Z',
    resolution: 60,
    online_min: 0,
    online_max: 0,
    online: 0,
    session_at: '2026-10-28T01:00:05Z',
    gap: false,
    drop: false,
    samples: 1,
  };
  vi.mocked(monitoringAPI.current).mockImplementation(
    async (_p, _a, _d, scope) =>
      response({
        total: scope === 'versions' ? 17 : 1,
        read: {
          scope,
          started_at: '',
          ended_at: '2026-10-28T01:00:05Z',
          duration_ns: 0,
          compared_devices: 17,
          version_divergence: false,
          package_divergence: true,
          rows:
            scope === 'versions'
              ? Array.from({ length: 16 }, (_, i) => ({
                  kind: 'version',
                  name: 'synthetic-' + i,
                  value: '7.14.3',
                  uptime: { known: false, min: 0, max: 0 },
                }))
              : [
                  {
                    kind: 'point',
                    point,
                    uptime: { known: false, min: 0, max: 0 },
                  },
                ],
        },
      }),
  );
  vi.mocked(readAPI.start).mockResolvedValue(
    response({ total: 0, job: { id: 'op', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'op', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'operations',
        started_at: '2026-10-28T01:00:00Z',
        ended_at: '2026-10-28T01:00:05Z',
        duration_ns: 5e9,
        availability: {
          services: 'unavailable',
          packages: 'complete',
          auth: 'partial',
        },
        rows: [
          {
            kind: 'resource',
            health: {
              cpu: 0,
              uptime: { known: false, min: 0, max: 0 },
              clock_offset: { known: false, min: 0, max: 0 },
            },
            uptime: { known: false, min: 0, max: 0 },
          },
        ],
      },
    }),
  );
  const view = render(
    <DeviceMonitoring profile="company" activation="active" device={device} />,
  );
  await screen.findByRole('img', {
    name: 'Curva de sessões online observadas',
  });
  expect(readAPI.start).not.toHaveBeenCalled();
  expect(screen.getByText('0 / 0 / 0')).toBeInTheDocument();
  expect(
    screen.getByText('Pontos da curva').closest('details'),
  ).not.toHaveAttribute('open');
  await screen.findByText(/Divergência de pacotes nos conjuntos completos/);
  expect(monitoringAPI.current).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    'versions',
    '',
  );
  expect(
    screen.queryByText(/Divergência de versões nas leituras/),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Atualizar monitoramento' }),
  );
  await screen.findByText('0 %');
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    expect.any(String),
    'operations',
    '',
    [],
    2,
  );
  expect(
    screen.getAllByText('Serviços telnet/ftp/www: indisponível.')[0],
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Falhas de autenticação PPP: leitura parcial/),
  ).toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  view.unmount();
});
it('reopening an expired local page returns to the first cursor', async () => {
  vi.mocked(monitoringAPI.current).mockImplementation(
    async (_p, _a, _d, scope, cursor) =>
      cursor
        ? { ...response(), code: 'CURSOR_INVALID' }
        : response({
            total: 17,
            next_cursor: 'expired-page',
            read: {
              scope,
              started_at: '',
              ended_at: '2026-10-28T01:00:00Z',
              duration_ns: 0,
              rows: [],
            },
          }),
  );
  const view = render(
    <DeviceMonitoring profile="company" activation="active" device={device} />,
  );
  fireEvent.click(
    (
      await screen.findAllByRole('button', {
        name: 'Próxima página dos dados locais',
      })
    )[0],
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reabrir série local' }),
  );
  await waitFor(() =>
    expect(vi.mocked(monitoringAPI.current).mock.calls.at(-1)?.[4]).toBe(''),
  );
  await screen.findAllByRole('button', {
    name: 'Próxima página dos dados locais',
  });
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  view.unmount();
});
it('a minute can disclose both a gap and an observed drop', async () => {
  vi.mocked(monitoringAPI.current).mockResolvedValue(
    response({
      total: 1,
      read: {
        scope: 'series',
        started_at: '',
        ended_at: '2026-10-28T01:00:05Z',
        duration_ns: 0,
        rows: [
          {
            kind: 'point',
            uptime: { known: false, min: 0, max: 0 },
            point: {
              at: '2026-10-28T01:00:00Z',
              resolution: 60,
              online_min: 0,
              online_max: 10,
              online: 0,
              gap: true,
              drop: true,
              samples: 2,
            },
          },
        ],
      },
    }),
  );
  const view = render(
    <DeviceMonitoring profile="company" activation="active" device={device} />,
  );
  await screen.findByText('Lacuna de continuidade · Queda brusca observada');
  view.unmount();
});
it('L06T: automatic traffic starts once with visible IDs; stop preserves graph and exit cancels retained job', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'traffic-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'traffic-job', state: 'RUNNING', code: 'OK' },
      read: {
        scope: 'traffic',
        started_at: '2026-10-28T01:00:00Z',
        ended_at: '2026-10-28T01:00:05Z',
        duration_ns: 5e9,
        rows: [
          {
            kind: 'session',
            row_id: 'visible-row',
            login: 'synthetic',
            uptime: { known: true, min: 1e9, max: 2e9 },
            download: { min: 1600, max: 1601 },
            upload: { min: 800, max: 801 },
            traffic: {
              points: [{ t: 5000, d: 1600, u: 800 }],
              download_bytes: '1000',
              upload_bytes: '500',
              covered_ns: 5e9,
              gaps: 0,
              average_download: { min: 1600, max: 1601 },
              average_upload: { min: 800, max: 801 },
              peak_download: { min: 1600, max: 1601 },
              peak_upload: { min: 800, max: 801 },
            },
          },
        ],
      },
    }),
  );
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  const { unmount } = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="current"
      rows={['visible-row']}
      traffic
    />,
  );
  await waitFor(() => expect(readAPI.start).toHaveBeenCalledTimes(1));
  expect(vi.mocked(readAPI.start).mock.calls[0].slice(4, 8)).toEqual([
    'traffic',
    'current',
    ['visible-row'],
    2,
  ]);
  await screen.findByRole('img', { name: 'Gráfico de tráfego de synthetic' });
  expect(screen.getByText(/pico amostrado/i)).toBeInTheDocument();
  expect(
    screen.getByRole('region', { name: 'Resumo de tráfego de synthetic' }),
  ).toHaveAttribute('title', 'Maior amostra: 1.600 bit/s');
  expect(screen.getAllByTitle('1.600–1.601 bit/s').length).toBeGreaterThan(0);
  expect(
    screen.getByTitle('1.000 bytes download / 500 bytes upload'),
  ).toHaveTextContent('1.000 B / 500 B');
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await screen.findByText('Observação parada.');
  expect(
    screen.getByRole('img', { name: 'Gráfico de tráfego de synthetic' }),
  ).toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  unmount();
  await waitFor(() =>
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'traffic-job',
    ),
  );
});
it('L06T: uncertain start is cancelled on context change and BUSY permits explicit retry after its automatic limit', async () => {
  vi.useFakeTimers();
  const late = deferred();
  vi.mocked(readAPI.start)
    .mockReturnValueOnce(late.promise)
    .mockResolvedValue({ ...response(), code: 'BUSY' });
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  const props = {
    profile: 'company',
    activation: 'active',
    device,
    observation: 'reading',
    rows: ['row'],
    traffic: true,
  };
  const view = render(<PPPoEReadPanel {...props} />);
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(readAPI.start).toHaveBeenCalledTimes(1);
    const operation = vi.mocked(readAPI.start).mock.calls[0][3];
    view.rerender(<PPPoEReadPanel {...props} rows={['other-row']} />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(readAPI.cancelOperation).toHaveBeenCalledWith(
      'company',
      'active',
      'device',
      operation,
    );
    expect(readAPI.start).toHaveBeenCalledTimes(2);
    await act(async () =>
      late.resolve(
        response({
          total: 0,
          job: { id: 'late-traffic', state: 'RUNNING', code: 'OK' },
        }),
      ),
    );
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'late-traffic',
    );
    expect(readAPI.start).toHaveBeenCalledTimes(2);
    for (const delay of [5000, 15000, 30000])
      await act(async () => {
        await vi.advanceTimersByTimeAsync(delay);
      });
    expect(readAPI.start).toHaveBeenCalledTimes(5);
    expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(readAPI.start).toHaveBeenCalledTimes(6);
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});
it('L06T: stop has no deadline or silent renewal after sixty seconds', async () => {
  vi.useFakeTimers();
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'traffic', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'traffic', state: 'RUNNING', code: 'OK' },
    }),
  );
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="reading"
      rows={['row']}
      traffic
    />,
  );
  try {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
    await act(async () => {
      await Promise.resolve();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(61000);
    });
    expect(screen.getByText('Observação parada.')).toBeInTheDocument();
    expect(
      screen.queryByText(/Prazo da observação encerrado/),
    ).not.toBeInTheDocument();
    expect(readAPI.start).toHaveBeenCalledTimes(1);
    expect(readAPI.stop).toHaveBeenCalledWith('company', 'active', 'traffic');
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});
for (const terminal of [false, true]) {
  it(`L06T: ${terminal ? 'empty terminal stops without renewal' : 'pending first sample has no invented freshness'}`, async () => {
    vi.useFakeTimers();
    let done = false;
    vi.mocked(readAPI.start).mockResolvedValue(
      response({
        total: 0,
        job: { id: 'pending-traffic', state: 'RUNNING', code: 'OK' },
      }),
    );
    vi.mocked(readAPI.status).mockImplementation(async () =>
      response({
        total: 0,
        job: {
          id: 'pending-traffic',
          state: done ? 'SUCCEEDED' : 'RUNNING',
          code: 'OK',
        },
      }),
    );
    vi.mocked(readAPI.cancel).mockResolvedValue(response());
    const view = render(
      <PPPoEReadPanel
        profile="company"
        activation="active"
        device={device}
        observation="reading"
        rows={['row']}
        traffic
      />,
    );
    try {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      if (!terminal) {
        expect(
          screen.getByText(/Aguardando primeira amostra\./),
        ).toBeInTheDocument();
        expect(
          screen.queryByText(/Última atualização há/),
        ).not.toBeInTheDocument();
      } else {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(61000);
        });
        expect(
          screen.queryByText(/Prazo da observação encerrado/),
        ).not.toBeInTheDocument();
        done = true;
        await act(async () => {
          await vi.advanceTimersByTimeAsync(350);
        });
        expect(
          screen.getByText('Leitura de tráfego sem amostras.'),
        ).toBeInTheDocument();
        expect(readAPI.start).toHaveBeenCalledTimes(1);
      }
    } finally {
      view.unmount();
      vi.useRealTimers();
    }
  });
}
