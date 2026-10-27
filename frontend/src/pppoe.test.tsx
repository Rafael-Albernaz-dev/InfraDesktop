import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoE } from './pppoe';
import {
  pppoeAPI as api,
  pppoeDiagnosticAPI as diagnosticAPI,
} from './presentation';
import type { PPResult, PPRow } from './presentation';
import type { Device } from './equipment-api';
vi.mock('./presentation', () => ({
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
  vi.mocked(diagnosticAPI.current).mockResolvedValue(result());
  vi.mocked(diagnosticAPI.list).mockResolvedValue(result());
  vi.mocked(api.current).mockResolvedValue(result({ current: obs, total: 2 }));
  vi.mocked(api.references).mockResolvedValue(
    result({ references: [], total: 0 }),
  );
  vi.mocked(api.list).mockResolvedValue(result({ rows, total: 2 }));
});
function deferredPP() {
  let resolve!: (r: PPResult) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<PPResult>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
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

it('L06C-UX: references have their own tab and session states have an explanation', async () => {
  vi.mocked(api.current).mockResolvedValue(
    result({ current: { ...obs, online: 0, offline: 0 }, total: 2 }),
  );
  showPP();
  await screen.findByRole('button', { name: 'Detalhar duplicado row-1' });
  expect(
    screen.queryByRole('heading', { name: 'Referências preservadas' }),
  ).not.toBeInTheDocument();
  expect(screen.getByText('2 ONLINE')).toBeInTheDocument();
  expect(
    screen.getByText(/ONLINE: vista na última leitura concluída/),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  expect(
    await screen.findByRole('heading', { name: 'Referências preservadas' }),
  ).toBeVisible();
  expect(
    screen
      .getAllByRole('button')
      .filter((button) => button.classList.contains('primary')),
  ).toHaveLength(1);
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(api.start).not.toHaveBeenCalled();
});

it('UX-R01: references can recover an initial reading failure without remote collection', async () => {
  vi.mocked(api.current).mockResolvedValueOnce({
    ...result(),
    code: 'SERVICE_UNAVAILABLE',
  });
  showPP();
  await screen.findByRole('alert');
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  fireEvent.click(
    screen.getByRole('button', { name: 'Reabrir leitura armazenada' }),
  );
  await screen.findByRole('button', {
    name: 'Salvar referência desta leitura',
  });
  expect(screen.getByText(/Leitura atual:/)).toHaveTextContent('2 registros');
  expect(api.start).not.toHaveBeenCalled();
});
it.each(['success', 'error', 'reject'] as const)(
  'A-H01: late detail %s cannot restore a filtered intent or steal focus',
  async (kind) => {
    const old = deferredPP();
    vi.mocked(api.detail).mockReturnValueOnce(old.promise);
    showPP();
    const trigger = await screen.findByRole('button', {
      name: 'Detalhar duplicado row-1',
    });
    trigger.focus();
    fireEvent.click(trigger);
    const search = screen.getByLabelText('Buscar sessões');
    search.focus();
    fireEvent.change(search, { target: { value: 'nova intenção' } });
    await waitFor(() =>
      expect(vi.mocked(api.list).mock.calls.at(-1)?.[4]?.search).toBe(
        'nova intenção',
      ),
    );
    await act(async () => {
      if (kind === 'reject') old.reject(new Error('old transport'));
      else
        old.resolve(
          kind === 'success'
            ? result({ rows: [rows[0]], total: 1 })
            : { ...result(), code: 'OBSERVATION_UNAVAILABLE' },
        );
    });
    expect(
      screen.queryByRole('complementary', { name: 'Detalhes da sessão' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(search).toHaveFocus();
  },
);
it('A-H01: closing an existing detail invalidates its pending replacement, while a fresh detail still works', async () => {
  const old = deferredPP();
  vi.mocked(api.detail)
    .mockResolvedValueOnce(result({ rows: [rows[0]], total: 1 }))
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue(result({ rows: [rows[1]], total: 1 }));
  showPP();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  await screen.findByRole('complementary', { name: 'Detalhes da sessão' });
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Fechar detalhe' }));
  await act(async () => old.resolve(result({ rows: [rows[1]], total: 1 })));
  expect(
    screen.queryByRole('complementary', { name: 'Detalhes da sessão' }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  expect(
    await screen.findByRole('complementary', { name: 'Detalhes da sessão' }),
  ).toHaveTextContent('row-2');
});
it('A-H01: changing comparison reference rejects a late pair before requesting its second leg', async () => {
  const old = deferredPP();
  vi.mocked(api.references).mockResolvedValue(
    result({
      references: [
        {
          reference_id: 'ref-a',
          observation_id: 'base-a',
          created_at: '2026-10-24T00:00:00Z',
          count: 2,
        },
        {
          reference_id: 'ref-b',
          observation_id: 'base-b',
          created_at: '2026-10-24T01:00:00Z',
          count: 2,
        },
      ],
      total: 2,
    }),
  );
  vi.mocked(api.compare).mockResolvedValue(
    result({
      deltas: [
        {
          base_row: 'left',
          target_row: 'right',
          category: 'changed',
          reason: 'continuity_supported',
          address_changed: false,
          caller_changed: false,
        },
      ],
      total: 1,
    }),
  );
  vi.mocked(api.detail)
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue(result({ rows: [rows[1]], total: 1 }));
  showPP();
  await screen.findAllByText('duplicado');
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  fireEvent.change(screen.getByLabelText('Referência para comparar'), {
    target: { value: 'base-a' },
  });
  fireEvent.click(await screen.findByRole('button', { name: 'Detalhar par' }));
  fireEvent.change(screen.getByLabelText('Referência para comparar'), {
    target: { value: 'base-b' },
  });
  await act(async () => old.resolve(result({ rows: [rows[0]], total: 1 })));
  expect(api.detail).toHaveBeenCalledTimes(1);
  expect(
    screen.queryByRole('complementary', { name: 'Detalhes da comparação' }),
  ).not.toBeInTheDocument();
});
it.each(['none', 'loading', 'reject'] as const)(
  'A-H02: %s comparison/page cannot retain a sessions count or actionable cursor',
  async (phase) => {
    vi.mocked(api.list).mockResolvedValueOnce(
      result({ rows, total: 17, next_cursor: 'old-cursor' }),
    );
    showPP();
    await screen.findByRole('button', { name: 'Próxima página' });
    if (phase === 'none') {
      fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
    } else {
      const page = deferredPP();
      vi.mocked(api.list).mockReturnValueOnce(page.promise);
      fireEvent.change(screen.getByLabelText('Buscar sessões'), {
        target: { value: 'next' },
      });
      await screen.findByText('Carregando página armazenada…');
      if (phase === 'reject')
        await act(async () => page.reject(new Error('transport')));
    }
    expect(
      screen.queryByRole('button', { name: 'Próxima página' }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('status', { name: '17 resultados' }),
    ).not.toBeInTheDocument();
    expect(api.compare).not.toHaveBeenCalled();
  },
);
it('A-H01: closing an already open detail by clearing filters keeps focus on search', async () => {
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [rows[0]], total: 1 }),
  );
  showPP();
  const search = await screen.findByLabelText('Buscar sessões');
  fireEvent.change(search, { target: { value: 'duplicado' } });
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  await screen.findByRole('complementary', { name: 'Detalhes da sessão' });
  await act(async () =>
    fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' })),
  );
  expect(
    screen.queryByRole('complementary', { name: 'Detalhes da sessão' }),
  ).not.toBeInTheDocument();
  expect(search).toHaveFocus();
});
it('A-H01: newer detail wins when two queries resolve in reverse order', async () => {
  const first = deferredPP(),
    second = deferredPP();
  vi.mocked(api.detail)
    .mockReturnValueOnce(first.promise)
    .mockReturnValueOnce(second.promise);
  showPP();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  await act(async () => second.resolve(result({ rows: [rows[1]], total: 1 })));
  expect(
    screen.getByRole('complementary', { name: 'Detalhes da sessão' }),
  ).toHaveTextContent('row-2');
  await act(async () => first.resolve(result({ rows: [rows[0]], total: 1 })));
  expect(
    screen.getByRole('complementary', { name: 'Detalhes da sessão' }),
  ).toHaveTextContent('row-2');
});
it('A-H01: filter change keeps a durable pending save and its successful reconciliation', async () => {
  const saved = deferredPP();
  vi.mocked(api.save).mockReturnValueOnce(saved.promise);
  showPP();
  fireEvent.click(
    await screen.findByRole('button', {
      name: 'Salvar referência desta leitura',
    }),
  );
  fireEvent.change(screen.getByLabelText('Buscar sessões'), {
    target: { value: 'new query' },
  });
  await act(async () =>
    saved.resolve(result({ effect_state: 'committed', total: 0 })),
  );
  expect(
    await screen.findByText('Referência salva da leitura exibida.'),
  ).toBeInTheDocument();
  expect(api.save).toHaveBeenCalledTimes(1);
});
it('A-H01/H02: open pair/reference and pending deletion respect the latest selection and its focus', async () => {
  const deleted = deferredPP();
  vi.mocked(api.references).mockResolvedValue(
    result({
      references: [
        {
          reference_id: 'ref-a',
          observation_id: 'base-a',
          created_at: '2026-10-24T00:00:00Z',
          count: 2,
        },
        {
          reference_id: 'ref-b',
          observation_id: 'base-b',
          created_at: '2026-10-24T01:00:00Z',
          count: 2,
        },
      ],
      total: 2,
    }),
  );
  vi.mocked(api.compare).mockResolvedValue(
    result({
      deltas: [
        {
          base_row: 'left',
          target_row: 'right',
          category: 'changed',
          reason: 'continuity_supported',
          address_changed: false,
          caller_changed: false,
        },
      ],
      total: 1,
    }),
  );
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [rows[0]], total: 1 }),
  );
  vi.mocked(api.remove).mockReturnValueOnce(deleted.promise);
  showPP();
  await screen.findAllByText('duplicado');
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  fireEvent.click(
    screen.getByRole('button', { name: 'Excluir referência ref-a' }),
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  const selection = screen.getByLabelText('Referência para comparar');
  fireEvent.change(selection, { target: { value: 'base-a' } });
  fireEvent.click(await screen.findByRole('button', { name: 'Detalhar par' }));
  await screen.findByRole('complementary', { name: 'Detalhes da comparação' });
  selection.focus();
  fireEvent.change(selection, { target: { value: 'base-b' } });
  expect(selection).toHaveFocus();
  expect(
    screen.queryByRole('complementary', { name: 'Detalhes da comparação' }),
  ).not.toBeInTheDocument();
  await act(async () => deleted.resolve(result()));
  expect(selection).toHaveValue('base-b');
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
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
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
  ).toHaveTextContent('2 resultados');
});

it('L06C: fields, offline reason and previous page use stored data', async () => {
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
  await screen.findByText('OFFLINE');
  expect(
    screen.getByRole('columnheader', { name: 'Caller ID' }),
  ).toBeInTheDocument();
  expect(screen.getByText('1 s')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Próxima página' }));
  await screen.findByRole('button', { name: 'Página anterior' });
  fireEvent.click(screen.getByRole('button', { name: 'Página anterior' }));
  await screen.findByText('OFFLINE');
  expect(vi.mocked(api.list).mock.calls.at(-1)?.[5]).toBe('');
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  await screen.findByText('Ping indisponível: sessão OFFLINE.');
  expect(
    screen.getByRole('button', { name: 'Sondar IP pelo concentrador' }),
  ).toBeDisabled();
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
  await screen.findByText(/old-login · 198.51.100.1/);
  expect(screen.getByText(/new-login · 198.51.100.1/)).toBeInTheDocument();
  expect(screen.getByText(/Reconectada/)).toBeInTheDocument();
  expect(screen.queryByText(/internal-base-id/)).not.toBeInTheDocument();
});
