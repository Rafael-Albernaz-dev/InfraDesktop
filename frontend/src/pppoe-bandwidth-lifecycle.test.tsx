import { RadioModes } from './radio-contract';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import {
  PPPoEDiagnostics,
  PPPoEPing,
  PPPoEReadPanel,
} from './pppoe-diagnostics';

import {
  pppoeDiagnosticAPI as api,
  pppoeReadAPI as readAPI,
  pppoeAPI as sessions,
} from './presentation';
import { pendingPPPoE } from './equipment-receipts';
import type { Device } from './equipment-api';
import type { PDDiagnostic, PDRow, PPResult } from './presentation';
vi.mock('./presentation', () => ({
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
function panel(scope: 'catalog' | 'radius' = 'catalog') {
  return render(
    <PPPoEDiagnostics
      profile="company"
      activation="active"
      device={device}
      scope={scope}
      blocked={false}
    />,
  );
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
it('L06C: lost ping response after worker exit never re-probes on reconcile', async () => {
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.ping).mockRejectedValueOnce(new Error('lost'));
  render(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="o"
      row="r"
      blocked={false}
    />,
  );
  await screen.findByText('Nenhuma sondagem armazenada para esta sessão.');
  fireEvent.click(
    screen.getByRole('button', { name: 'Sondar IP pelo concentrador' }),
  );
  await screen.findByRole('button', { name: 'Conferir andamento' });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir andamento' }));
  await screen.findByText(
    'Não há trabalho ativo desta ação. Inicie uma nova leitura deliberadamente.',
  );
  expect(api.ping).toHaveBeenCalledTimes(1);
});
it('L06C: catalog type filter resets paging and queries the stored full reading', async () => {
  panel();
  await screen.findByRole('cell', { name: 'Serviço de teste' });
  fireEvent.change(screen.getByLabelText('Filtrar por tipo'), {
    target: { value: 'service' },
  });
  await waitFor(() =>
    expect(vi.mocked(api.list).mock.calls.at(-1)?.slice(5)).toEqual([
      '',
      'service',
    ]),
  );
  expect(api.catalog).not.toHaveBeenCalled();
});
it('L06D-S2: one bandwidth action, explicit direction, unknown rate and legitimate zero', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'read-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'read-job', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'bandwidth',
        started_at: '2026-10-27T00:00:00Z',
        ended_at: '2026-10-27T00:00:02Z',
        duration_ns: 2e9,
        rows: [
          {
            kind: 'session',
            row_id: 'row',
            login: 'synthetic',
            uptime: { known: false, min: 0, max: 0 },
            upload: { min: 0, max: 0 },
          },
        ],
      },
    }),
  );
  render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  expect(screen.getAllByRole('button', { name: 'Medir banda' })).toHaveLength(
    1,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Medir banda' }));
  expect(await screen.findByText('0 bps')).toBeInTheDocument();
  expect(
    screen.getByRole('columnheader', {
      name: 'Download do assinante',
    }),
  ).toHaveAttribute('title', 'TX concentrador');
  expect(
    within(screen.getByText('synthetic').closest('tr')!).getAllByRole(
      'cell',
    )[1],
  ).toHaveTextContent('Não informado nesta leitura');
  expect(readAPI.start).toHaveBeenCalledTimes(1);
});
it('L06D-S2: leaving before start response cancels the late job', async () => {
  const start = deferred();
  vi.mocked(readAPI.start).mockReturnValue(start.promise);
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Medir banda' }));
  view.unmount();
  await act(async () =>
    start.resolve(
      response({
        total: 0,
        job: { id: 'late-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  await waitFor(() =>
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'late-job',
    ),
  );
  expect(readAPI.status).not.toHaveBeenCalled();
});
it('L06D-H-D05: revision change discards pending start and restores the action', async () => {
  const start = deferred();
  vi.mocked(readAPI.start).mockReturnValue(start.promise);
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Medir banda' }));
  view.rerender(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={{ ...device, revision: device.revision + 1 }}
      observation="observation"
      rows={['row']}
    />,
  );
  expect(screen.getByRole('button', { name: 'Medir banda' })).toBeEnabled();
  await act(async () =>
    start.resolve(
      response({
        total: 0,
        job: { id: 'obsolete-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  await waitFor(() =>
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'obsolete-job',
    ),
  );
  expect(readAPI.status).not.toHaveBeenCalled();
});
it('L06D-S3: live is explicit, calculated uptime is labelled and exit cancels it', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'live-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'live-job', state: 'RUNNING', code: 'OK' },
      read: {
        scope: RadioModes.live,
        started_at: new Date().toISOString(),
        ended_at: new Date().toISOString(),
        duration_ns: 2e9,
        rows: [
          {
            kind: 'session',
            row_id: 'row',
            login: 'synthetic',
            uptime: { known: true, min: 1e9, max: 2e9 },
            download: { min: 1000, max: 1100 },
            upload: { min: 500, max: 600 },
          },
        ],
      },
    }),
  );
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
  expect(await screen.findByText(/\(calculado\)/)).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Encerrar ao vivo' }),
  ).toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'live',
    'observation',
    ['row'],
    device.revision,
  );
  view.unmount();
  await waitFor(() =>
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'live-job',
    ),
  );
});
it('L06D-S3: exit during uncertain start sends the exact operation cancellation', async () => {
  const start = deferred();
  vi.mocked(readAPI.start).mockReturnValue(start.promise);
  vi.mocked(readAPI.cancelOperation).mockResolvedValue(response());
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
  const operation = vi.mocked(readAPI.start).mock.calls[0][3];
  view.unmount();
  await waitFor(() =>
    expect(readAPI.cancelOperation).toHaveBeenCalledWith(
      'company',
      'active',
      device.id,
      operation,
    ),
  );
  await act(async () =>
    start.resolve(
      response({
        total: 0,
        job: { id: 'late-live', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  await waitFor(() =>
    expect(readAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'late-live',
    ),
  );
});
it('L06D-H-D08: uncertain live cannot reuse its operation for bandwidth', async () => {
  vi.mocked(readAPI.start).mockRejectedValue(
    new Error('synthetic lost start response'),
  );
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
  await screen.findByText(/Não foi possível confirmar a solicitação/);
  expect(screen.getByRole('button', { name: 'Medir banda' })).toBeDisabled();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  view.unmount();
});
it('L06D-H-D09: lost live status never extrapolates forever beyond60s', async () => {
  vi.useFakeTimers();
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: RadioModes.live, state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  vi.mocked(readAPI.status)
    .mockResolvedValueOnce(
      response({
        total: 1,
        job: { id: RadioModes.live, state: 'RUNNING', code: 'OK' },
        read: {
          scope: RadioModes.live,
          started_at: new Date().toISOString(),
          ended_at: new Date().toISOString(),
          duration_ns: 2e9,
          rows: [
            {
              kind: 'session',
              login: 'synthetic',
              uptime: { known: true, min: 1e9, max: 2e9 },
            },
          ],
        },
      }),
    )
    .mockRejectedValue(new Error('synthetic status unavailable'));
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      rows={['row']}
    />,
  );
  try {
    fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
    await act(async () => {
      await Promise.resolve();
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByText(/\(calculado\)/)).toBeInTheDocument();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(61000);
    });
    expect(screen.queryByText(/\(calculado\)/)).not.toBeInTheDocument();
    expect(
      screen.getByText(/Prazo da observação encerrado/),
    ).toBeInTheDocument();
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});
