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
  DeviceMonitoring,
  PPPoEPing,
  PPPoEReadPanel,
} from './pppoe-diagnostics';
import { PPPoE } from './pppoe';
import {
  pppoeDiagnosticAPI as api,
  deviceMonitoringAPI as monitoringAPI,
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

it('opens and pages persisted catalog without remote refresh; rejected page has no old cursor or false empty', async () => {
  vi.mocked(api.list).mockResolvedValueOnce(
    response({ diagnostic_rows: rows, next_cursor: 'signed-page', total: 17 }),
  );
  panel();
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  expect(api.catalog).not.toHaveBeenCalled();
  expect(api.radius).not.toHaveBeenCalled();
  expect(api.ping).not.toHaveBeenCalled();
  const page = deferred();
  vi.mocked(api.list).mockReturnValueOnce(page.promise);
  fireEvent.click(
    screen.getByRole('button', { name: 'Próxima página do diagnóstico' }),
  );
  expect(
    screen.queryByRole('button', { name: 'Próxima página do diagnóstico' }),
  ).not.toBeInTheDocument();
  await act(async () => page.reject(new Error('transport')));
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Não foi possível carregar a página.',
  );
  expect(
    screen.queryByText('Nenhum item configurado nesta leitura completa.'),
  ).not.toBeInTheDocument();
  expect(vi.mocked(api.list).mock.calls.at(-1)?.slice(2)).toEqual([
    'device',
    'catalog',
    'catalog-snapshot',
    'signed-page',
    '',
  ]);
  fireEvent.click(
    screen.getByRole('button', { name: 'Reabrir dados armazenados' }),
  );
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  expect(api.catalog).not.toHaveBeenCalled();
});

it('reconciles an uncertain start through stored state without a second collection', async () => {
  vi.mocked(api.catalog)
    .mockRejectedValueOnce(new Error('lost receipt'))
    .mockResolvedValue(
      response({ job: { id: 'job', state: 'RUNNING', code: 'OK' }, total: 0 }),
    );
  vi.mocked(api.status).mockImplementation(async () =>
    response({
      job: {
        id: 'job',
        state: 'SUCCEEDED',
        code: 'OK',
        pppoe: { operation_id: vi.mocked(api.catalog).mock.calls[0][3] },
      },
      total: 0,
    }),
  );
  panel();
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  fireEvent.click(
    screen.getByRole('button', { name: 'Atualizar serviços e VLANs' }),
  );
  await screen.findByRole('alert');
  expect(api.catalog).toHaveBeenCalledTimes(1);
  const operation = vi.mocked(api.catalog).mock.calls[0][3];
  vi.mocked(api.current).mockResolvedValueOnce(
    response({
      attempt: {
        operation_id: operation,
        state: 'RUNNING',
        job_id: 'job',
        code: 'OK',
        quality: { status: 'complete', counts_known: true, eligible: 1 },
      },
      total: 0,
    }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Consultar ação pendente' }),
  );
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Atualizar serviços e VLANs' }),
    ).toBeEnabled(),
  );
  expect(api.catalog).toHaveBeenCalledTimes(1);
  expect(api.current).toHaveBeenCalled();
  expect(pendingPPPoE.size).toBe(0);
});

it('recovers status explicitly without replaying the network action', async () => {
  vi.mocked(api.radius).mockResolvedValue(
    response({ job: { id: 'job', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(api.status)
    .mockRejectedValueOnce(new Error('status unavailable'))
    .mockResolvedValue(
      response({
        job: { id: 'job', state: 'SUCCEEDED', code: 'OK' },
        total: 0,
      }),
    );
  panel('radius');
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar RADIUS' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar estado novamente' }),
  );
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
  await waitFor(() =>
    expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
  );
  expect(api.radius).toHaveBeenCalledTimes(1);
});

it('a late catalog response cannot populate a newer RADIUS surface', async () => {
  const old = deferred();
  vi.mocked(api.current).mockReturnValueOnce(old.promise);
  const view = panel();
  vi.mocked(api.list).mockResolvedValue(
    response({
      diagnostic_rows: [
        { id: 'aaa', kind: 'aaa', use_radius: false, accounting: true },
      ],
      total: 1,
    }),
  );
  view.rerender(
    <PPPoEDiagnostics
      profile="company"
      activation="active"
      device={device}
      scope="radius"
      blocked={false}
    />,
  );
  await screen.findByRole('heading', { name: 'PPP AAA' });
  await act(async () =>
    old.resolve(response({ diagnostic: meta('catalog'), total: 1 })),
  );
  expect(screen.getByRole('heading', { name: 'PPP AAA' })).toBeInTheDocument();
  expect(
    screen.queryByRole('heading', { name: 'Serviço de teste' }),
  ).not.toBeInTheDocument();
});

it('does not show a different session ping and sends only immutable source IDs', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      diagnostic: {
        ...meta('ping'),
        source_observation: 'observation',
        source_row: 'other-row',
      },
      total: 1,
    }),
  );
  vi.mocked(api.ping).mockResolvedValue(
    response({
      job: { id: 'ping-job', state: 'RUNNING', code: 'OK' },
      total: 0,
    }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      job: {
        id: 'ping-job',
        state: 'FAILED',
        code: 'OBSERVATION_INCOMPATIBLE',
      },
      total: 0,
    }),
  );
  render(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      row="selected-row"
      blocked={false}
    />,
  );
  await screen.findByText('Nenhuma sondagem armazenada para esta sessão.');
  expect(api.list).not.toHaveBeenCalled();
  expect(screen.queryByText(/Recebidos:/)).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Sondar IP pelo concentrador' }),
  );
  await screen.findByRole('alert');
  expect(api.ping).toHaveBeenCalledExactlyOnceWith(
    'company',
    'active',
    'device',
    'observation',
    'selected-row',
    expect.any(String),
    2,
  );
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
});

it('keeps no-response RTT unknown and preserves RADIUS decimal precision', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      diagnostic: {
        ...meta('ping'),
        source_observation: 'observation',
        source_row: 'row',
      },
      total: 1,
    }),
  );
  vi.mocked(api.list).mockResolvedValue(
    response({
      diagnostic_rows: [
        {
          id: 'ping-row',
          kind: 'ping',
          ping: {
            sent: 3,
            received: 0,
            loss_percent: 100,
            context: 'concentrador · main',
            probes: [0, 1, 2].map((sequence) => ({
              sequence,
              status: 'timeout',
            })),
          },
        },
      ],
      total: 1,
    }),
  );
  const view = render(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      row="row"
      blocked={false}
    />,
  );
  await screen.findByText(/Enviados: 3 · Recebidos: 0 · Perda: 100 %/);
  expect(screen.getByText(/RTT mínimo:/)).toHaveTextContent(
    'RTT mínimo: Indisponível · médio: Indisponível · máximo: Indisponível',
  );
  view.unmount();
  vi.mocked(api.current).mockResolvedValue(
    response({ diagnostic: meta('radius'), total: 1 }),
  );
  vi.mocked(api.list).mockResolvedValue(
    response({
      diagnostic_rows: [
        {
          id: 'radius-row',
          kind: 'radius',
          address: '198.51.100.20',
          counters: {
            pending: '0',
            requests: '18446744073709551615',
            accepts: '1',
            rejects: '2',
            resends: '0',
            timeouts: '0',
            bad_replies: '0',
            last_rtt: { known: true, min: 1000000, max: 1999999 },
          },
        },
      ],
      total: 1,
    }),
  );
  panel('radius');
  expect(
    await screen.findByText('18.446.744.073.709.551.615'),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/não identificam motivo de rejeição/),
  ).toBeInTheDocument();
});

it('makes new scopes available before session collection with one primary action and no references controls', async () => {
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'VLANs e serviços' }));
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  expect(
    screen.getAllByRole('button', { name: 'Atualizar serviços e VLANs' }),
  ).toHaveLength(1);
  expect(
    screen.queryByRole('button', { name: 'Atualizar sessões' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('heading', { name: 'Referências preservadas' }),
  ).not.toBeInTheDocument();
  expect(api.catalog).not.toHaveBeenCalled();
  expect(sessions.start).not.toHaveBeenCalled();
});

it('does not adopt another session running ping or stale uncertain presentation', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      attempt: {
        state: 'RUNNING',
        code: 'OK',
        job_id: 'job-A',
        quality: { status: 'complete', counts_known: true, eligible: 0 },
        operation_id: 'operation-A',
        source_observation: 'observation',
        source_row: 'row-A',
      },
      total: 0,
    }),
  );
  const receiptKey = 'company|active|device|diag|ping|2|observation|row-A';
  pendingPPPoE.set(receiptKey, { operation: 'operation-A', revision: 2 });
  const view = render(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      row="row-B"
      blocked={false}
    />,
  );
  await screen.findByText('Nenhuma sondagem armazenada para esta sessão.');
  expect(
    screen.queryByRole('button', { name: 'Cancelar sondagem' }),
  ).not.toBeInTheDocument();
  expect(api.status).not.toHaveBeenCalled();
  expect(pendingPPPoE.get(receiptKey)?.operation).toBe('operation-A');
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.ping).mockRejectedValueOnce(new Error('lost response'));
  fireEvent.click(
    screen.getByRole('button', { name: 'Sondar IP pelo concentrador' }),
  );
  await screen.findByRole('button', { name: 'Consultar sondagem pendente' });
  const size = pendingPPPoE.size;
  view.rerender(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="new-observation"
      row="row-C"
      blocked={false}
    />,
  );
  expect(
    await screen.findByRole('button', { name: 'Sondar IP pelo concentrador' }),
  ).toBeEnabled();
  expect(pendingPPPoE.size).toBe(size);
  expect(api.ping).toHaveBeenCalledTimes(1);
});

it.each([
  'OBSERVATION_INCOMPATIBLE',
  'OBSERVATION_UNAVAILABLE',
  'BUSY',
  'LIMIT_EXCEEDED',
])('clears a definitively rejected start receipt: %s', async (code) => {
  vi.mocked(api.catalog).mockResolvedValue({ ...response(), code });
  const view = panel();
  await screen.findByRole('heading', { name: 'Serviço de teste' });
  fireEvent.click(
    screen.getByRole('button', { name: 'Atualizar serviços e VLANs' }),
  );
  await screen.findByRole('alert');
  expect(pendingPPPoE.size).toBe(0);
  expect(
    screen.queryByRole('button', { name: 'Consultar ação pendente' }),
  ).not.toBeInTheDocument();
  view.rerender(
    <PPPoEDiagnostics
      profile="company"
      activation="active"
      device={{ ...device, revision: 3 }}
      scope="catalog"
      blocked={false}
    />,
  );
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Atualizar serviços e VLANs' }),
    ).toBeEnabled(),
  );
  expect(api.catalog).toHaveBeenCalledTimes(1);
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
  await screen.findByRole('button', { name: 'Consultar sondagem pendente' });
  fireEvent.click(
    screen.getByRole('button', { name: 'Consultar sondagem pendente' }),
  );
  await screen.findByText(
    'Não há trabalho ativo desta ação. Inicie uma nova leitura deliberadamente.',
  );
  expect(api.ping).toHaveBeenCalledTimes(1);
});
it('L06C: catalog type filter resets paging and queries the stored full reading', async () => {
  panel();
  await screen.findByRole('heading', { name: 'Serviço de teste' });
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
  expect(
    screen.getAllByRole('button', { name: 'Medir banda do assinante' }),
  ).toHaveLength(1);
  fireEvent.click(
    screen.getByRole('button', { name: 'Medir banda do assinante' }),
  );
  expect(await screen.findByText('0 bps')).toBeInTheDocument();
  expect(
    screen.getByRole('columnheader', {
      name: 'Download do assinante (TX concentrador)',
    }),
  ).toBeInTheDocument();
  expect(
    within(screen.getByText('synthetic').closest('tr')!).getAllByRole(
      'cell',
    )[1],
  ).toHaveTextContent('Indisponível');
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Medir banda do assinante' }),
  );
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Medir banda do assinante' }),
  );
  view.rerender(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={{ ...device, revision: device.revision + 1 }}
      observation="observation"
      rows={['row']}
    />,
  );
  expect(
    screen.getByRole('button', { name: 'Medir banda do assinante' }),
  ).toBeEnabled();
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
        scope: 'live',
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Observar ao vivo por até 60 s' }),
  );
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Observar ao vivo por até 60 s' }),
  );
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Observar ao vivo por até 60 s' }),
  );
  await screen.findByText(/Não foi possível confirmar a solicitação/);
  expect(
    screen.getByRole('button', { name: 'Medir banda do assinante' }),
  ).toBeDisabled();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  view.unmount();
});
it('L06D-H-D09: lost live status never extrapolates forever beyond60s', async () => {
  vi.useFakeTimers();
  vi.mocked(readAPI.start).mockResolvedValue(
    response({ total: 0, job: { id: 'live', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  vi.mocked(readAPI.status)
    .mockResolvedValueOnce(
      response({
        total: 1,
        job: { id: 'live', state: 'RUNNING', code: 'OK' },
        read: {
          scope: 'live',
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
    fireEvent.click(
      screen.getByRole('button', { name: 'Observar ao vivo por até 60 s' }),
    );
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

it('L06D-S4: top is deliberate across all tunnels and its page never starts another read', async () => {
  const meta = {
    scope: 'top',
    started_at: '2026-10-27T00:00:00Z',
    ended_at: '2026-10-27T00:00:02Z',
    duration_ns: 2e9,
    cost: {
      ssh_commands: 2,
      ssh_bytes: 2048,
      first_tunnels: 17,
      last_tunnels: 17,
    },
    rows: [
      {
        kind: 'session',
        login: 'first-page',
        uptime: { known: false, min: 0, max: 0 },
      },
    ],
  };
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'top-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 17,
      job: { id: 'top-job', state: 'SUCCEEDED', code: 'OK' },
      read: meta,
      next_cursor: 'next-page',
    }),
  );
  vi.mocked(readAPI.page).mockResolvedValue(
    response({
      total: 17,
      read: {
        ...meta,
        rows: [
          {
            kind: 'session',
            login: 'second-page',
            uptime: { known: false, min: 0, max: 0 },
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
      observation="old-observation"
      rows={[]}
      allowTop
    />,
  );
  expect(
    screen.queryByRole('button', { name: 'Medir banda da página' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getAllByRole('button', { name: 'Buscar top consumo' }),
  ).toHaveLength(1);
  fireEvent.click(screen.getByRole('button', { name: 'Buscar top consumo' }));
  await screen.findByText('first-page');
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'top',
    '',
    [],
    device.revision,
  );
  expect(
    screen.getByText(/2 comandos SSH · 17\/17 túneis/),
  ).toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Próxima página de consumo' }),
  );
  await screen.findByText('second-page');
  expect(screen.queryByText('first-page')).not.toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  expect(readAPI.page).toHaveBeenCalledWith(
    'company',
    'active',
    'top-job',
    'next-page',
  );
});

it('L06D-S4: complete empty top has an honest empty state and no reset control', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'top-empty', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'top-empty', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'top',
        started_at: '2026-10-27T00:00:00Z',
        ended_at: '2026-10-27T00:00:02Z',
        duration_ns: 2e9,
        rows: [],
      },
    }),
  );
  render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation=""
      rows={[]}
      allowTop
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Buscar top consumo' }));
  expect(
    await screen.findByText('Nenhum túnel ativo na leitura concluída.'),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Início do consumo' }),
  ).not.toBeInTheDocument();
});

it('L06D-S5: selected log has projected categories and no irrelevant bandwidth controls offline', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'log-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'log-job', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'log',
        cost: {
          ssh_commands: 1,
          ssh_bytes: 128,
          first_tunnels: 0,
          last_tunnels: 0,
        },
        started_at: '2026-10-27T00:00:00Z',
        ended_at: '2026-10-27T00:00:01Z',
        duration_ns: 1e9,
        rows: [
          {
            kind: 'log',
            at: '01:00:00',
            event: 'auth_failed',
            uptime: { known: false, min: 0, max: 0 },
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
      observation="current"
      rows={[]}
      logRow="offline-row"
    />,
  );
  expect(
    screen.queryByRole('button', { name: 'Medir banda da página' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Consultar log PPP' }));
  await screen.findByText('Falha de autenticação');
  expect(
    screen.getByRole('columnheader', { name: 'Horário no equipamento' }),
  ).toBeInTheDocument();
  expect(screen.queryByText(/túneis nas amostras/)).not.toBeInTheDocument();
  expect(
    screen.queryByRole('columnheader', {
      name: 'Download do assinante (TX concentrador)',
    }),
  ).not.toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'log',
    'current',
    ['offline-row'],
    device.revision,
  );
});

it('L06D ajuste2: selected secret is ephemeral, explicit about unavailable fields and cleared on context change', async () => {
  vi.mocked(readAPI.cancel).mockResolvedValue(response());
  vi.mocked(readAPI.start).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'secret-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 2,
      job: { id: 'secret-job', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'secret',
        started_at: '2026-10-27T00:00:00Z',
        ended_at: '2026-10-27T00:00:01Z',
        duration_ns: 1e9,
        rows: [
          {
            kind: 'secret',
            uptime: { known: false, min: 0, max: 0 },
            secret: {
              disabled: true,
              profile: 'default',
              comment: 'Texto de teste <script>literal</script>',
            },
          },
          {
            kind: 'secret',
            uptime: { known: false, min: 0, max: 0 },
            secret: { disabled: true, comment: '' },
          },
        ],
      },
    }),
  );
  const props = {
    profile: 'company',
    activation: 'active',
    device,
    observation: 'current',
    rows: [],
    logRow: 'offline-row',
  };
  const { rerender } = render(<PPPoEReadPanel {...props} />);
  fireEvent.click(
    screen.getByRole('button', { name: 'Ver evidência do secret' }),
  );
  const evidence = await screen.findByRole('region', {
    name: 'Evidência efêmera do secret',
  });
  expect(
    within(evidence).getByText(
      'Texto livre do administrador no roteador; pode conter dado pessoal e não é motivo verificado',
    ),
  ).toBeInTheDocument();
  expect(
    within(evidence).getByText('Texto de teste <script>literal</script>'),
  ).toBeInTheDocument();
  expect(within(evidence).getAllByText('Sim')).toHaveLength(2);
  expect(within(evidence).getAllByText('Indisponível').length).toBeGreaterThan(
    0,
  );
  expect(within(evidence).getByText('Sem texto informado')).toBeInTheDocument();
  expect(screen.queryByText(/túneis nas amostras/)).not.toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'secret',
    'current',
    ['offline-row'],
    device.revision,
  );
  rerender(<PPPoEReadPanel {...props} logRow="other-row" />);
  expect(readAPI.cancel).toHaveBeenCalledWith(
    'company',
    'active',
    'secret-job',
  );
  expect(
    screen.queryByRole('region', { name: 'Evidência efêmera do secret' }),
  ).not.toBeInTheDocument();
});

it('L06D-S6: pools keep unknown separate from zero and interfaces have their own directions', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({ total: 0, job: { id: 'health', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 2,
      job: { id: 'health', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'health',
        pools_available: true,
        uplinks_available: true,
        pool_total: 1,
        uplink_total: 1,
        started_at: '2026-10-27T00:00:00Z',
        ended_at: '2026-10-27T00:00:02Z',
        duration_ns: 2e9,
        rows: [
          {
            kind: 'pool',
            name: 'pool',
            used: 0,
            size: 3,
            uptime: { known: false, min: 0, max: 0 },
          },
          {
            kind: 'uplink',
            name: 'ether1',
            tx: { min: 3478.2608, max: 4000 },
            rx: { min: 0, max: 0 },
            rx_errors: '0',
            tx_drops: '18446744073709551615',
            uptime: { known: false, min: 0, max: 0 },
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
      observation=""
      rows={[]}
      health
    />,
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Consultar pools e interfaces' }),
  );
  expect(await screen.findByText('pool')).toBeInTheDocument();
  const int = screen.getByText('ether1').closest('tr')!;
  expect(within(int).getByText('3,47–4 Kbps')).toBeInTheDocument();
  expect(within(int).getByText('0 bps')).toBeInTheDocument();
  expect(
    within(int).getByText('18.446.744.073.709.551.615'),
  ).toBeInTheDocument();
  expect(within(int).getAllByText('Indisponível').length).toBeGreaterThan(0);
  expect(
    screen.getByRole('columnheader', { name: 'TX da interface' }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Medir banda da página' }),
  ).not.toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'health',
    '',
    [],
    device.revision,
  );
});

for (const stage of ['start', 'terminal'])
  it(`L06D-H-D15: unsupported log at ${stage} reports capability unavailable and recovers`, async () => {
    vi.mocked(readAPI.start).mockResolvedValue(
      stage === 'start'
        ? { ...response(), code: 'UNSUPPORTED' }
        : response({
            total: 0,
            job: { id: 'unsupported-log', state: 'RUNNING', code: 'OK' },
          }),
    );
    vi.mocked(readAPI.status).mockResolvedValue(
      response({
        total: 0,
        job: { id: 'unsupported-log', state: 'FAILED', code: 'UNSUPPORTED' },
      }),
    );
    render(
      <PPPoEReadPanel
        profile="company"
        activation="active"
        device={device}
        observation="current"
        rows={[]}
        logRow="row"
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Consultar log PPP' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Esta leitura não está disponível neste formato, versão ou permissão.',
    );
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Consultar log PPP' }),
      ).toBeEnabled(),
    );
  });

it('L06T2: local series has explicit unknowns; global package divergence survives pagination and remote refresh is deliberate', async () => {
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
  fireEvent.click(screen.getByRole('tab', { name: 'Versões' }));
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
    screen.getByText('Serviços telnet/ftp/www: indisponível.'),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Falhas de autenticação PPP: leitura parcial/),
  ).toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledTimes(1);
  view.unmount();
});

it('L06T2: reopening an expired local page returns to the first cursor', async () => {
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
    await screen.findByRole('button', {
      name: 'Próxima página dos dados locais',
    }),
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reabrir série local' }),
  );
  await waitFor(() =>
    expect(vi.mocked(monitoringAPI.current).mock.calls.at(-1)?.[4]).toBe(''),
  );
  await screen.findByRole('button', {
    name: 'Próxima página dos dados locais',
  });
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  view.unmount();
});

it('L06T2: a minute can disclose both a gap and an observed drop', async () => {
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

it('L06T: entering traffic starts once with visible IDs; stop preserves graph and exit cancels retained job', async () => {
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
    screen.getByText('Escala: 0–1,6 Kbps · download / upload'),
  ).toHaveAttribute('title', '1.600 bit/s');
  expect(screen.getAllByTitle('1.600–1.601 bit/s').length).toBeGreaterThan(0);
  expect(
    screen.getByTitle('1.000 bytes download / 500 bytes upload'),
  ).toHaveTextContent('1.000 B download / 500 B upload');
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

it('L06T: uncertain auto-start is cancelled on context change and BUSY has only explicit retry', async () => {
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
  await waitFor(() => expect(readAPI.start).toHaveBeenCalledTimes(1));
  const operation = vi.mocked(readAPI.start).mock.calls[0][3];
  view.rerender(<PPPoEReadPanel {...props} rows={['other-row']} />);
  await waitFor(() =>
    expect(readAPI.cancelOperation).toHaveBeenCalledWith(
      'company',
      'active',
      'device',
      operation,
    ),
  );
  await waitFor(() => expect(readAPI.start).toHaveBeenCalledTimes(2));
  await screen.findByRole('button', { name: 'Iniciar nova observação' });
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Iniciar nova observação' }),
  );
  await waitFor(() => expect(readAPI.start).toHaveBeenCalledTimes(3));
  view.unmount();
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
  it(`L06T: ${terminal ? 'confirmed terminal clears deadline warning' : 'pending first sample has no invented freshness'}`, async () => {
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
          screen.getByText(/Prazo da observação encerrado/),
        ).toBeInTheDocument();
        done = true;
        fireEvent.click(
          screen.getByRole('button', { name: 'Consultar estado da leitura' }),
        );
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1);
        });
        expect(
          screen.queryByText(/Prazo da observação encerrado/),
        ).not.toBeInTheDocument();
        expect(readAPI.start).toHaveBeenCalledTimes(1);
      }
    } finally {
      view.unmount();
      vi.useRealTimers();
    }
  });
}

it.each([
  [
    'COLLECTOR_DEFECT',
    'O:syntax',
    'Falha do coletor do InfraDesktop neste firmware.',
  ],
  [
    'PARTIAL',
    'P:echo-mismatch',
    'Não foi possível concluir esta leitura. Tente novamente.',
  ],
])(
  'L06V7: terminal %s exposes only the structural technical reason',
  async (code, reason, message) => {
    vi.mocked(readAPI.start).mockResolvedValue(
      response({ total: 0, job: { id: 'op', state: 'RUNNING', code: 'OK' } }),
    );
    vi.mocked(readAPI.status).mockResolvedValue({
      ...response({ total: 0, job: { id: 'op', state: 'FAILED', code } }),
      error: { reason },
    });
    render(
      <PPPoEReadPanel
        profile="company"
        activation="active"
        device={device}
        observation=""
        rows={[]}
        monitoring
      />,
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Atualizar monitoramento' }),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent(message);
    fireEvent.click(screen.getByText('Detalhe técnico'));
    expect(screen.getByText(`Motivo: ${reason}`)).toBeVisible();
    expect(screen.queryByText('Resultado operacional')).not.toBeInTheDocument();
  },
);

it('L06V7: real zero auth count is explicit and never inferred from an empty page', async () => {
  const result = {
    scope: 'operations',
    started_at: '',
    ended_at: '2026-10-28T01:00:00Z',
    duration_ns: 0,
    availability: { auth: 'complete' },
    rows: [
      {
        kind: 'availability',
        name: 'auth',
        value: 'complete',
        used: 0,
        size: 0,
        uptime: { known: false, min: 0, max: 0 },
      },
    ],
  };
  vi.mocked(readAPI.start).mockResolvedValue(
    response({ total: 0, job: { id: 'op', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 1,
      job: { id: 'op', state: 'SUCCEEDED', code: 'OK' },
      read: result,
    }),
  );
  render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation=""
      rows={[]}
      monitoring
    />,
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Atualizar monitoramento' }),
  );
  expect(
    await screen.findByText(
      'Nenhuma falha de autenticação reconhecida no buffer examinado.',
    ),
  ).toBeVisible();
});

it('L06V7: empty page without an auth counter does not claim zero', async () => {
  vi.mocked(readAPI.start).mockResolvedValue(
    response({ total: 0, job: { id: 'op', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(readAPI.status).mockResolvedValue(
    response({
      total: 0,
      job: { id: 'op', state: 'SUCCEEDED', code: 'OK' },
      read: {
        scope: 'operations',
        started_at: '',
        ended_at: '2026-10-28T01:00:00Z',
        duration_ns: 0,
        availability: { auth: 'complete' },
        rows: [],
      },
    }),
  );
  render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation=""
      rows={[]}
      monitoring
    />,
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Atualizar monitoramento' }),
  );
  await screen.findByRole('region', { name: 'Resultado operacional' });
  expect(
    screen.queryByText(
      'Nenhuma falha de autenticação reconhecida no buffer examinado.',
    ),
  ).not.toBeInTheDocument();
});

it('L06T3: absent dynamic interfaces are separate and never presented as running', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({ diagnostic: meta('catalog'), total: 2 }),
  );
  vi.mocked(api.list).mockResolvedValue(
    response({
      total: 2,
      diagnostic_rows: [
        {
          id: 'present',
          kind: 'interface',
          name: 'X-current',
          type: 'pppoe-in',
          dynamic: true,
          running: true,
        },
        {
          id: 'absent',
          kind: 'interface',
          name: 'X-absent',
          type: 'pppoe-in',
          dynamic: true,
          running: true,
          not_seen_since: '2026-10-28T17:00:00Z',
        },
      ],
    }),
  );
  panel();
  await screen.findByRole('heading', { name: 'X-absent' });
  const absent = screen
    .getByRole('heading', { name: 'X-absent' })
    .closest('article')!;
  expect(within(absent).getByText(/Não visto desde/)).toBeInTheDocument();
  expect(
    within(absent).queryByText(/Em execução: Sim/),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('region', {
      name: 'Interfaces dinâmicas não vistas na leitura atual',
    }),
  ).toContainElement(absent);
  const current = screen
    .getByRole('heading', { name: 'X-current' })
    .closest('article')!;
  expect(within(current).getByText(/Em execução: Sim/)).toBeInTheDocument();
});

it('L06T3: cross concentrator data stays local and distant readings stay non comparable', async () => {
  vi.mocked(monitoringAPI.current).mockImplementation(
    async (_p, _a, _d, scope) =>
      response({
        total: scope === 'duplicates' ? 1 : 0,
        read: {
          scope,
          started_at: '',
          ended_at: '2026-10-28T17:00:00Z',
          duration_ns: 0,
          rows:
            scope === 'duplicates'
              ? [
                  {
                    kind: 'availability',
                    name: 'X-peer',
                    value: 'não comparável (leituras distantes)',
                    uptime: { known: false, min: 0, max: 0 },
                  },
                ]
              : [],
        },
      }),
  );
  render(
    <DeviceMonitoring profile="company" activation="active" device={device} />,
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Entre concentradores' }));
  expect(
    await screen.findByText(/X-peer: não comparável \(leituras distantes\)/),
  ).toBeInTheDocument();
  expect(readAPI.start).not.toHaveBeenCalled();
});

it('L06T3: ping shows jitter, previous10 baseline and the3 probe sample with exact titles', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      diagnostic: {
        ...meta('ping'),
        source_observation: 'observation',
        source_row: 'row',
      },
      total: 1,
    }),
  );
  vi.mocked(api.list).mockResolvedValue(
    response({
      total: 1,
      diagnostic_rows: [
        {
          id: 'ping',
          kind: 'ping',
          ping: {
            sent: 3,
            received: 3,
            loss_percent: 0,
            min_ns: 1e6,
            avg_ns: 2e6,
            max_ns: 3e6,
            jitter_ns: 1e6,
            baseline_median_ns: 1e6,
            baseline_samples: 10,
            context: 'main',
            probes: [0, 1, 2].map((sequence) => ({
              sequence,
              status: '',
              rtt_ns: (sequence + 1) * 1e6,
            })),
          },
        },
      ],
    }),
  );
  render(
    <PPPoEPing
      profile="company"
      activation="active"
      device={device}
      observation="observation"
      row="row"
      blocked={false}
    />,
  );
  expect(await screen.findByText(/Amostra de 3 probes/)).toHaveTextContent(
    'jitter: 1 ms',
  );
  expect(screen.getByText(/Mediana dos 10 pings/)).toHaveTextContent(
    'RTT médio +100 %',
  );
  expect(screen.getAllByTitle('1.000.000 ns').length).toBeGreaterThan(0);
});
