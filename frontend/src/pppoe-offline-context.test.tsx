import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoE } from './pppoe';
import {
  deviceMonitoringAPI as monitoringAPI,
  pppoeAPI as api,
  pppoeReadAPI as readAPI,
  pppoeDiagnosticAPI as diagnosticAPI,
} from './presentation';
import type { PPResult, PPRow } from './presentation';
import type { Device } from './equipment-api';
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
    current: vi.fn(),
    list: vi.fn(),
    catalog: vi.fn(),
    radius: vi.fn(),
    ping: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
  },
  pppoeAPI: {
    groups: vi.fn(),
    current: vi.fn(),
    references: vi.fn(),
    list: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    detail: vi.fn(),
    save: vi.fn(),
    remove: vi.fn(),
    compare: vi.fn(),
    export: vi.fn(),
    exportOperational: vi.fn(),
  },
}));
const device = {
  id: 'device',
  name: 'Concentrador sintético',
  revision: 2,
  active: true,
  fingerprint: 'pinned',
} as Device;
const obs = {
  observation_id: 'observation-a',
  count: 2,
  provenance: { ended_at: '2026-10-25T00:00:00Z' },
  quality: { status: 'complete' },
};
const rows: PPRow[] = ['row-1', 'row-2'].map((row_id) => ({
  row_id,
  remote_id: row_id,
  session_id: row_id,
  login: 'duplicado',
  address: '198.51.100.1',
  caller_id: '02:00:00:00:00:01',
  uptime: { min: 1000000000, max: 1999999999, known: true },
  duplicates: 2,
}));
const result = (data: PPResult['data'] = { total: 0 }): PPResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data,
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(monitoringAPI.current).mockResolvedValue(result());
  vi.mocked(readAPI.start).mockResolvedValue({ ...result(), code: 'BUSY' });
  vi.mocked(readAPI.cancel).mockResolvedValue(result());
  vi.mocked(readAPI.cancelOperation).mockResolvedValue(result());
  vi.mocked(readAPI.stop).mockResolvedValue(result());
  vi.mocked(api.groups).mockResolvedValue(result({ groups: [], total: 0 }));
  vi.mocked(diagnosticAPI.current).mockResolvedValue(result());
  vi.mocked(diagnosticAPI.list).mockResolvedValue(result());
  vi.mocked(api.current).mockResolvedValue(result({ current: obs, total: 2 }));
  vi.mocked(api.references).mockResolvedValue(
    result({ references: [], total: 0 }),
  );
  vi.mocked(api.list).mockResolvedValue(result({ rows, total: 2 }));
});

function showPP() {
  return render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
}
it('L06D-S1: stable context shows negotiated IPv6 and unavailable fields without invented zero', async () => {
  const detail = {
    ...rows[0],
    context: {
      association: 'associated',
      service: 'service',
      vlan: 100,
      ipv6: true,
      pd_status: 'none',
      profile: 'local-profile',
      secret_state: 'active',
      upload_limit: '5000000',
      download_limit: '10000000',
    },
  };
  vi.mocked(api.detail).mockResolvedValue(result({ rows: [detail], total: 1 }));
  vi.mocked(api.groups).mockResolvedValue(
    result({ groups: [{ service: 'service', vlan: 100, count: 2 }], total: 1 }),
  );
  showPP();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  fireEvent.click(screen.getByText(/Mais informações/));
  expect(await screen.findByText('Sim')).toBeInTheDocument();
  expect(screen.getByText('Sem prefixo delegado')).toBeInTheDocument();
  expect(screen.getAllByText('10 Mbps / 5 Mbps').find((element) => element.closest('.kv'))).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  expect(
    within((await screen.findByRole('cell', { name: 'service' })).closest('tr')!)
      .getAllByRole('cell')
      .map((cell) => cell.textContent),
  ).toEqual(['100', 'service', '2', 'Não informado nesta leitura']);
});
it.each([
  ['disabled', 'Secret local desativado: a autenticação não vem dele'],
  ['ambiguous', 'Vários secrets locais ativos: perfil ambíguo'],
  ['none', 'Nenhum secret local para este login'],
  ['unavailable', 'Não lido ou falha na leitura do secret local'],
  ['', 'Não lido ou falha na leitura do secret local'],
])(
  'L06D ajuste2: %s never presents an unverified local profile',
  async (state, label) => {
    vi.mocked(api.detail).mockResolvedValue(
      result({
        rows: [
          {
            ...rows[0],
            context: {
              association: 'associated',
              secret_state: state,
              profile: 'disabled-default-canary',
              upload_limit: '5000000',
              download_limit: '10000000',
            },
          },
        ],
        total: 1,
      }),
    );
    showPP();
    fireEvent.click(
      await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
    );
    fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
    fireEvent.click(screen.getByText(/Mais informações/));
    expect(await screen.findByText(label)).toBeInTheDocument();
    expect(
      screen.queryByText('disabled-default-canary'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText('Perfil do secret ativo'),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText('Limite de plano (fila dinâmica) — download / upload'),
    ).toBeInTheDocument();
    expect(screen.getAllByText('10 Mbps / 5 Mbps').find((element) => element.closest('.kv'))).toBeVisible();
    expect(
      screen.getAllByRole('button', { name: 'Ver evidência do secret' }),
    ).toHaveLength(1);
  },
);
it('L06D-H-D10: a concentrator with no stored observation can request fresh top', async () => {
  vi.mocked(api.current).mockResolvedValue(result({ total: 0 }));
  showPP();
  await screen.findByText(/Nenhuma leitura de sessões disponível/);
  expect(screen.getAllByRole('button', { name: 'Top consumo' })).toHaveLength(
    1,
  );
  expect(
    screen.queryByRole('button', { name: 'Medir banda da página' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Salvar referência desta leitura' }),
  ).not.toBeInTheDocument();
});
it('L06D-S7: operational CSV uses the exact observation and returns focus after native cancellation', async () => {
  vi.mocked(api.exportOperational).mockResolvedValue('EXPORT_CANCELLED');
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  const action = await screen.findByRole('button', {
    name: 'Exportar CSV operacional',
  });
  expect(action).toHaveAttribute(
    'title',
    'O CSV contém dados identificáveis de assinantes.',
  );
  action.focus();
  fireEvent.click(action);
  await waitFor(() =>
    expect(api.exportOperational).toHaveBeenCalledWith(
      'company',
      'active',
      device.id,
      obs.observation_id,
    ),
  );
  await waitFor(() => expect(action).toBeEnabled());
  expect(action).toHaveFocus();
  expect(
    screen.queryByText('Arquivo CSV operacional salvo.'),
  ).not.toBeInTheDocument();
  expect(api.export).not.toHaveBeenCalled();
});
it('L06T3: FastTrack marks every displayed plan limit without a router action', async () => {
  vi.mocked(monitoringAPI.current).mockResolvedValue(
    result({
      total: 1,
      read: {
        scope: 'equipment',
        started_at: '',
        ended_at: '2026-10-28T17:00:00Z',
        duration_ns: 0,
        rows: [
          {
            kind: 'resource',
            uptime: { known: false, min: 0, max: 0 },
            health: {
              fasttrack_rules: 1,
              uptime: { known: false, min: 0, max: 0 },
              clock_offset: { known: false, min: 0, max: 0 },
            },
          },
        ],
      },
    }),
  );
  vi.mocked(api.detail).mockResolvedValue(
    result({
      total: 1,
      rows: [
        {
          ...rows[0],
          context: {
            association: 'associated',
            download_limit: '100000000',
            upload_limit: '50000000',
          },
        },
      ],
    }),
  );
  showPP();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  expect(
    (await screen.findAllByText('limite lido, não garantido')).find((element) => element.closest('.kv')),
  ).toBeInTheDocument();
  expect(screen.getAllByText('100 Mbps / 50 Mbps').find((element) => element.closest('.kv'))).toHaveAttribute(
    'title',
    '100.000.000 bit/s download / 50.000.000 bit/s upload',
  );
  expect(
    screen.queryByRole('button', { name: /desativar FastTrack/i }),
  ).not.toBeInTheDocument();
});
it('L06T3: VLAN peak uses raw counts, a configurable drop threshold and optional plan column', async () => {
  vi.mocked(api.groups).mockResolvedValue(
    result({
      total: 1,
      groups: [{ service: 'X-service', vlan: 100, count: 4, peak_7d: 10 }],
    }),
  );
  vi.mocked(api.list).mockResolvedValue(
    result({
      total: 1,
      rows: [
        {
          ...rows[0],
          context: {
            association: 'associated',
            download_limit: '100000000',
            upload_limit: '50000000',
          },
        },
      ],
    }),
  );
  showPP();
  const group = (
    await screen.findByRole('cell', { name: 'X-service' })
  ).closest('tr')!;
  expect(
    within(group)
      .getAllByRole('cell')
      .map((cell) => cell.textContent),
  ).toEqual(['100', 'X-service', '4 · Queda local detectada', '10']);
  expect(within(group).getByText('10')).toHaveAttribute('title', 'Δ −60 %');
  expect(screen.getByText('· Queda local detectada')).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText('Queda local'), {
    target: { value: '75' },
  });
  expect(screen.queryByText('· Queda local detectada')).not.toBeInTheDocument();
  fireEvent.click(screen.getByLabelText('Mostrar plano/limite'));
  expect(
    screen.getByRole('columnheader', { name: 'Plano' }),
  ).toBeInTheDocument();
  expect(screen.getByText('100 Mbps / 50 Mbps').closest('td')).toHaveAttribute(
    'title',
    '100.000.000 bit/s download / 50.000.000 bit/s upload',
  );
});
