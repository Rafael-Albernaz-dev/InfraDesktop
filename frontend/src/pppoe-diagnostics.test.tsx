import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoEDiagnostics, PPPoEPing } from './pppoe-diagnostics';
import { PPPoE } from './pppoe';
import {
  pppoeDiagnosticAPI as api,
  pppoeAPI as sessions,
} from './presentation';
import { pendingPPPoE } from './equipment-receipts';
import type { Device } from './equipment-api';
import type { PDDiagnostic, PDRow, PPResult } from './presentation';

vi.mock('./presentation', () => ({
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
  await screen.findByText(/Enviados: 3 · Recebidos: 0 · Perda: 100%/);
  expect(screen.getByText(/RTT mínimo: Indisponível/)).toHaveTextContent(
    'médio: Indisponível',
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
  expect(await screen.findByText('18446744073709551615')).toBeInTheDocument();
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
