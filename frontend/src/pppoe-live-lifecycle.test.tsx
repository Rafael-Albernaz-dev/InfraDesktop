import { RadioCodes } from './radio-contract';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from './pppoe-diagnostics';

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
  expect(screen.getAllByRole('button', { name: 'Top consumo' })).toHaveLength(
    1,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Top consumo' }));
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
  fireEvent.click(screen.getByRole('button', { name: 'Top consumo' }));
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
  fireEvent.click(screen.getByRole('button', { name: 'Log PPP' }));
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
  expect(
    within(evidence).getAllByText('Não informado nesta leitura').length,
  ).toBeGreaterThan(0);
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
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  expect(await screen.findByText('pool')).toBeInTheDocument();
  const int = screen.getByText('ether1').closest('tr')!;
  expect(within(int).getByText('3,47–4 Kbps')).toBeInTheDocument();
  expect(within(int).getByText('0 bps')).toBeInTheDocument();
  expect(
    within(int).getByText('18.446.744.073.709.551.615'),
  ).toBeInTheDocument();
  expect(
    within(int).getAllByText('Não informado nesta leitura').length,
  ).toBeGreaterThan(0);
  expect(screen.getByRole('columnheader', { name: 'TX' })).toHaveAttribute(
    'title',
    'TX da interface',
  );
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
        ? { ...response(), code: RadioCodes.UNSUPPORTED }
        : response({
            total: 0,
            job: { id: 'unsupported-log', state: 'RUNNING', code: 'OK' },
          }),
    );
    vi.mocked(readAPI.status).mockResolvedValue(
      response({
        total: 0,
        job: {
          id: 'unsupported-log',
          state: 'FAILED',
          code: RadioCodes.UNSUPPORTED,
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
        logRow="row"
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Log PPP' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Esta leitura não está disponível neste formato, versão ou permissão.',
    );
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Log PPP' })).toBeEnabled(),
    );
  });
