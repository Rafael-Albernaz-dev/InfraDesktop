import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { DeviceMonitoring } from '../../features/pppoe/device-monitoring';
import {
  PPPoEDiagnostics,
  PPPoEPing,
  PPPoEReadPanel,
} from '../../features/pppoe/pppoe-diagnostics';

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
  await screen.findByText(
    'Nenhuma falha de autenticação reconhecida no buffer examinado.',
  );
  fireEvent.click(
    screen
      .getByRole('region', { name: 'Resultado operacional' })
      .querySelector('summary')!,
  );
  expect(
    screen.getByText(
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
  const absent = (
    await screen.findByRole('cell', { name: 'X-absent' })
  ).closest('tr')!;
  expect(within(absent).getAllByRole('cell')[1]).toHaveAttribute(
    'title',
    '2026-10-28T17:00:00Z',
  );
  expect(absent).not.toHaveTextContent(/Em execução: Sim/);
  expect(
    screen.getByText('Não vistas na leitura atual').closest('.block'),
  ).toContainElement(absent);
  const current = screen.getByRole('cell', { name: 'X-current' });
  expect(current).toHaveAttribute(
    'title',
    expect.stringContaining('Em execução: Sim'),
  );
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
  await screen.findByText('Entre concentradores');
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
