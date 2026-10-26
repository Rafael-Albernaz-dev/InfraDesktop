import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoE } from './pppoe';
import { pppoeAPI as api } from './presentation';
import type { PPResult, PPRow } from './presentation';
import type { Device } from './equipment-api';
vi.mock('./presentation', () => ({
  pppoeAPI: {
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
  vi.mocked(api.current).mockResolvedValue(result({ current: obs, total: 2 }));
  vi.mocked(api.references).mockResolvedValue(
    result({ references: [], total: 0 }),
  );
  vi.mocked(api.list).mockResolvedValue(result({ rows, total: 2 }));
});
it('opens stored reading without collection, preserves duplicate rows and saves displayed ID', async () => {
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findAllByText('duplicado');
  expect(screen.getAllByText('duplicado')).toHaveLength(2);
  expect(api.start).not.toHaveBeenCalled();
  expect(
    screen.getAllByRole('button', { name: 'Atualizar sessões' }),
  ).toHaveLength(1);
  vi.mocked(api.save).mockResolvedValue(result());
  fireEvent.click(
    screen.getByRole('button', { name: 'Salvar referência desta leitura' }),
  );
  await waitFor(() => expect(api.save).toHaveBeenCalled());
  expect(vi.mocked(api.save).mock.calls[0].slice(0, 4)).toEqual([
    'company',
    'active',
    'device',
    'observation-a',
  ]);
  expect(api.start).not.toHaveBeenCalled();
});
it('filters and recovers locally, keeps page and detail IDs immutable', async () => {
  vi.mocked(api.list).mockResolvedValue(
    result({ rows, total: 17, next_cursor: 'signed-page' }),
  );
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [rows[1]], total: 1 }),
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
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  await screen.findByRole('complementary', { name: 'Detalhes da sessão' });
  expect(api.detail).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    'observation-a',
    'row-2',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Próxima página' }));
  await waitFor(() =>
    expect(vi.mocked(api.list).mock.calls.at(-1)?.at(-1)).toBe('signed-page'),
  );
  vi.mocked(api.list).mockResolvedValue(result({ rows: [], total: 0 }));
  fireEvent.change(screen.getByLabelText('Buscar sessões'), {
    target: { value: 'sem resultado' },
  });
  await screen.findByText('Nenhuma sessão corresponde aos filtros.');
  expect(
    screen.getAllByRole('button', { name: 'Limpar filtros' }),
  ).toHaveLength(1);
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  await waitFor(() =>
    expect(screen.getByLabelText('Buscar sessões')).toHaveValue(''),
  );
  expect(api.start).not.toHaveBeenCalled();
});
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
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar sessões' }));
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
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar sessões' }));
  await screen.findByRole('alert');
  fireEvent.click(
    screen.getByRole('button', { name: 'Consultar estado novamente' }),
  );
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
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
  const remove = screen.getByRole('button', {
    name: 'Excluir referência ref-focu',
  });
  remove.focus();
  fireEvent.click(remove);
  await waitFor(() => expect(remove).not.toBeInTheDocument());
  expect(
    screen.getByRole('heading', { name: 'Referências preservadas' }),
  ).toHaveFocus();
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
  ).toHaveTextContent('2 resultados');
});
