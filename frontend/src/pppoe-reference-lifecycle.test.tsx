import {
  act,
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
it('shows limitation over last good table and quota cause, local actions remain available', async () => {
  vi.mocked(api.current).mockResolvedValue(
    result({
      current: obs,
      attempt: {
        state: 'SUCCEEDED',
        code: 'OK',
        quality: { status: 'limited', counts_known: true, eligible: 50000 },
      },
      total: 2,
    }),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...result(),
    code: 'LIMIT_EXCEEDED',
    data: undefined,
    error: { reason: 'operation_receipts' },
  });
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findByText(/A tabela continua sendo a última leitura boa/);
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  await screen.findByRole('alert');
  expect(screen.getByRole('alert')).toHaveTextContent('2048');
  expect(
    screen.getByRole('button', { name: 'Exportar JSON sanitizado' }),
  ).toBeEnabled();
});
it('ignores late data after unmount/context switch and picker cancellation is neutral', async () => {
  let resolve!: (r: PPResult) => void;
  vi.mocked(api.current).mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  const view = render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  view.unmount();
  await act(async () => resolve(result({ current: obs, total: 2 })));
  expect(api.list).not.toHaveBeenCalled();
  expect(api.start).not.toHaveBeenCalled();
});
it('review: failed page is not a successful empty search', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    code: 'SERVICE_UNAVAILABLE',
    data: undefined,
  });
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findByRole('alert');
  expect(screen.queryByText('0 resultados')).not.toBeInTheDocument();
  expect(
    screen.queryByText('Nenhuma sessão corresponde aos filtros.'),
  ).not.toBeInTheDocument();
});
it('review: failed status can recover the existing job without a new collection', async () => {
  vi.mocked(api.start).mockResolvedValue(
    result({
      job: { id: 'recover-job', state: 'RUNNING', code: 'OK' },
      total: 0,
    }),
  );
  vi.mocked(api.status)
    .mockRejectedValueOnce(new Error('transport'))
    .mockResolvedValue(
      result({
        job: { id: 'recover-job', state: 'SUCCEEDED', code: 'OK' },
        total: 0,
      }),
    );
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  await screen.findByRole('alert');
  fireEvent.click(screen.getByRole('button', { name: 'Conferir andamento' }));
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
  await waitFor(() =>
    expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
});
it('identifies comparison pairs and opens both immutable source rows', async () => {
  vi.mocked(api.references).mockResolvedValue(
    result({
      references: [
        {
          reference_id: 'ref',
          observation_id: 'base',
          created_at: '2026-10-24T00:00:00Z',
          count: 2,
        },
      ],
      total: 1,
    }),
  );
  vi.mocked(api.compare).mockResolvedValue(
    result({
      deltas: [
        {
          base_row: 'base-row',
          target_row: 'target-row',
          category: 'changed',
          reason: 'continuity_supported',
          address_changed: true,
          caller_changed: true,
        },
      ],
      total: 1,
    }),
  );
  vi.mocked(api.detail).mockImplementation(async (_p, _a, _d, o, r) =>
    result({
      rows: [
        {
          ...rows[0],
          row_id: r,
          login: o === 'base' ? 'login-base' : 'login-target',
        },
      ],
      total: 1,
    }),
  );
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  fireEvent.change(screen.getByLabelText('Referência para comparar'), {
    target: { value: 'base' },
  });
  await screen.findByRole('button', { name: 'Detalhar par' });
  fireEvent.click(screen.getByRole('button', { name: 'Detalhar par' }));
  await screen.findByText('Login: login-base');
  expect(screen.getByText('Login: login-target')).toBeInTheDocument();
  expect(api.detail).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    'base',
    'base-row',
  );
  expect(api.detail).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    'observation-a',
    'target-row',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Fechar par' }));
  expect(screen.getByRole('button', { name: 'Detalhar par' })).toHaveFocus();
  expect(api.start).not.toHaveBeenCalled();
});
it('reports a removed replay honestly and cancellation of picker stays neutral', async () => {
  vi.mocked(api.save).mockResolvedValue(
    result({ effect_state: 'removed', total: 0 }),
  );
  vi.mocked(api.export).mockResolvedValue('EXPORT_CANCELLED');
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  fireEvent.click(
    screen.getByRole('button', { name: 'Salvar referência desta leitura' }),
  );
  await screen.findByText(
    'Esta referência já foi removida. O replay não a recriou.',
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Exportar JSON sanitizado' }),
  );
  await waitFor(() => expect(api.export).toHaveBeenCalled());
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  expect(api.start).not.toHaveBeenCalled();
});
it('review: initial stored-read failure offers local recovery without collecting', async () => {
  vi.mocked(api.current)
    .mockResolvedValueOnce({
      ...result(),
      code: 'SERVICE_UNAVAILABLE',
      data: undefined,
    })
    .mockResolvedValue(result({ current: obs, total: 2 }));
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findByRole('alert');
  expect(
    screen.queryByText(
      'Nenhuma leitura de sessões disponível. Atualize para consultar este concentrador.',
    ),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Reabrir leitura armazenada' }),
  );
  await screen.findAllByText('duplicado');
  expect(api.current).toHaveBeenCalledTimes(2);
  expect(api.start).not.toHaveBeenCalled();
});
it('review focus: disappearing clear/delete controls and cancelled picker restore useful focus', async () => {
  let retained = true;
  vi.mocked(api.references).mockImplementation(async () =>
    result({
      references: retained
        ? [
            {
              reference_id: 'ref-focus',
              observation_id: obs.observation_id,
              created_at: '2026-10-25T00:00:00Z',
              count: 2,
            },
          ]
        : [],
      total: retained ? 1 : 0,
    }),
  );
  vi.mocked(api.remove).mockImplementation(async () => {
    retained = false;
    return result();
  });
  vi.mocked(api.export).mockResolvedValue('EXPORT_CANCELLED');
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  const search = screen.getByLabelText('Buscar sessões');
  fireEvent.change(search, { target: { value: 'query' } });
  const clear = screen.getByRole('button', { name: 'Limpar filtros' });
  clear.focus();
  fireEvent.click(clear);
  expect(search).toHaveFocus();
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  const remove = screen.getByRole('button', {
    name: 'Excluir referência ref-focu',
  });
  remove.focus();
  fireEvent.click(remove);
  await waitFor(() => expect(remove).not.toBeInTheDocument());
  expect(
    screen.getByRole('heading', { name: 'Referências preservadas' }),
  ).toHaveFocus();
  fireEvent.click(screen.getByRole('tab', { name: 'Lista' }));
  await screen.findByRole('button', { name: 'Detalhar duplicado row-1' });
  const exporting = screen.getByRole('button', {
    name: 'Exportar JSON sanitizado',
  });
  exporting.focus();
  fireEvent.click(exporting);
  await waitFor(() => expect(exporting).toBeEnabled());
  expect(exporting).toHaveFocus();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  expect(api.start).not.toHaveBeenCalled();
});
it('review references: loading and failure do not claim an empty catalog; local retry recovers', async () => {
  let reply!: (r: PPResult) => void;
  vi.mocked(api.references).mockImplementationOnce(
    () =>
      new Promise((r) => {
        reply = r;
      }),
  );
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  expect(
    screen.queryByText('Nenhuma referência salva.'),
  ).not.toBeInTheDocument();
  await act(async () => reply({ ...result(), code: 'SERVICE_UNAVAILABLE' }));
  expect(
    screen.queryByText('Nenhuma referência salva.'),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Recarregar referências' }),
  );
  await screen.findByText('Nenhuma referência salva.');
  expect(api.references).toHaveBeenCalledTimes(2);
  expect(api.start).not.toHaveBeenCalled();
});
it('exposes the completed result count as an accessible status', async () => {
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  expect(
    screen.getByRole('status', { name: '2 resultados' }),
  ).toHaveAccessibleName('2 resultados');
});
it('L06C: fields, offline reason and previous page use stored data', async () => {
  const clock = vi
    .spyOn(Date, 'now')
    .mockReturnValue(Date.parse('2026-10-26T12:02:30Z'));
  const offline = {
    ...rows[0],
    state: 'OFFLINE',
    absent_start: '2026-10-26T12:00:00Z',
    absent_end: '2026-10-26T12:01:00Z',
  };
  vi.mocked(api.list).mockImplementation(async (_p, _a, _d, _o, _q, c) =>
    result({
      rows: c ? [rows[1]] : [offline],
      total: 2,
      next_cursor: c ? undefined : 'second',
    }),
  );
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [offline], total: 1 }),
  );
  showPP();
  await screen.findByRole('cell', { name: 'OFFLINE' });
  expect(
    screen.getByRole('columnheader', { name: 'Conectado há' }),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Próxima página' }));
  await screen.findByRole('button', { name: 'Página anterior' });
  expect(vi.mocked(api.list).mock.calls.at(-1)?.[5]).toBe('second');
  await screen.findByRole('cell', { name: 'ONLINE' });
  fireEvent.click(screen.getByRole('button', { name: 'Página anterior' }));
  await screen.findByRole('cell', { name: 'OFFLINE' });
  expect(vi.mocked(api.list).mock.calls.at(-1)?.[5]).toBe('');
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  await screen.findByText('Sem leitura: sessão OFFLINE');
  expect(screen.getByText('Caller ID').nextElementSibling).toHaveTextContent(
    '02:00:00:00:00:01',
  );
  fireEvent.click(screen.getByText(/Mais informações/));
  expect(screen.getByText('Offline desde (intervalo)')).toBeInTheDocument();
  expect(
    screen.getByText('Duração da última sessão').nextElementSibling,
  ).toHaveTextContent('1 s');
  expect(screen.getByText('Há quanto tempo (pelo menos)')).toBeInTheDocument();
  expect(screen.getByText(/1 min 30 s/)).toBeInTheDocument();
  clock.mockRestore();
  expect(screen.getByRole('button', { name: 'Ping' })).toBeDisabled();
  expect(
    screen.queryByText(/Identidade local da linha/),
  ).not.toBeInTheDocument();
  expect(diagnosticAPI.ping).not.toHaveBeenCalled();
});
it('L06C: comparison identifies each side without UUID labels', async () => {
  vi.mocked(api.references).mockResolvedValue(
    result({
      references: [
        {
          reference_id: 'ref',
          observation_id: 'base',
          created_at: '2026-10-25T00:00:00Z',
          count: 1,
        },
      ],
      total: 1,
    }),
  );
  vi.mocked(api.compare).mockResolvedValue(
    result({
      deltas: [
        {
          base_row: 'internal-base-id',
          target_row: 'internal-target-id',
          base: { ...rows[0], login: 'old-login' },
          target: { ...rows[1], login: 'new-login' },
          category: 'changed',
          reason: 'reconnected',
          address_changed: false,
          caller_changed: false,
        },
      ],
      total: 1,
    }),
  );
  showPP();
  await screen.findByRole('button', {
    name: 'Salvar referência desta leitura',
  });
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  fireEvent.change(screen.getByLabelText('Referência para comparar'), {
    target: { value: 'base' },
  });
  await screen.findByRole('cell', { name: 'new-login' });
  vi.mocked(api.detail).mockImplementation(async (_p, _a, _d, observation) =>
    result({
      rows: [
        {
          ...rows[0],
          login: observation === 'base' ? 'old-login' : 'new-login',
        },
      ],
      total: 1,
    }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Detalhar par' }));
  await screen.findByText(/old-login/);
  expect(
    within(
      screen.getByRole('complementary', { name: 'Detalhes da comparação' }),
    ).getByText('Login: new-login'),
  ).toBeInTheDocument();
  expect(screen.getAllByText(/198.51.100.1/)).toHaveLength(2);
  expect(screen.getByText(/Reconectada/)).toBeInTheDocument();
  expect(screen.queryByText(/internal-base-id/)).not.toBeInTheDocument();
});
