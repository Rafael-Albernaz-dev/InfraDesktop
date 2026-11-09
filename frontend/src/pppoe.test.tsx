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
it('budget-omitted context preserves sessions, explains detail and avoids unknown groups', async () => {
  vi.mocked(api.current).mockResolvedValue(
    result({ current: { ...obs, context_omitted: true }, total: 2 }),
  );
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [rows[0]], total: 1 }),
  );
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await screen.findByText(
    'Contexto não incluído por limite. As sessões e seus estados foram preservados.',
  );
  expect(
    await screen.findAllByRole('button', { name: /^Detalhar duplicado/ }),
  ).toHaveLength(2);
  expect(api.groups).not.toHaveBeenCalled();
  expect(screen.queryByText('Não associado')).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  fireEvent.click(screen.getByText(/Mais informações/));
  await screen.findByText(
    'Contexto não incluído por limite. VLAN, IPv6 e plano indisponíveis nesta leitura.',
  );
  expect(
    screen.queryByText(
      'Túnel: não associado. Dados de VLAN, IPv6 e plano indisponíveis.',
    ),
  ).not.toBeInTheDocument();
  expect(api.start).not.toHaveBeenCalled();
});
it('references keep their own tab and saving stays on the sessions list', async () => {
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
  expect(screen.getByRole('button', { name: 'Atualizar' })).toBeEnabled();
  expect(
    screen.queryByRole('button', { name: 'Salvar referência desta leitura' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Recarregar referências' }),
  ).toBeEnabled();
  fireEvent.click(screen.getByRole('tab', { name: 'Lista' }));
  expect(
    screen.getByRole('button', { name: 'Salvar referência desta leitura' }),
  ).toBeEnabled();
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
  await screen.findByText(/Leitura atual:/);
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
      screen.queryByRole('region', { name: 'Detalhes da sessão' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(search).toHaveFocus();
  },
);
it('A-H01: returning and changing the filter invalidates a pending detail, while a fresh detail still works', async () => {
  const old = deferredPP();
  vi.mocked(api.detail)
    .mockResolvedValueOnce(result({ rows: [rows[0]], total: 1 }))
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue(result({ rows: [rows[1]], total: 1 }));
  showPP();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  await screen.findByRole('region', { name: 'Detalhes da sessão' });
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  fireEvent.change(screen.getByLabelText('Buscar sessões'), {
    target: { value: 'nova intenção' },
  });
  await act(async () => old.resolve(result({ rows: [rows[1]], total: 1 })));
  expect(
    screen.queryByRole('region', { name: 'Detalhes da sessão' }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: 'Detalhar duplicado row-2' }),
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  expect(
    await screen.findByRole('region', { name: 'Detalhes da sessão' }),
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
      screen.getByRole('button', { name: 'Próxima página' }),
    ).toBeDisabled();
    expect(
      screen.queryByRole('status', { name: '17 resultados' }),
    ).not.toBeInTheDocument();
    expect(api.compare).not.toHaveBeenCalled();
  },
);
it('A-H01: returning to the list and clearing filters keeps focus on search', async () => {
  vi.mocked(api.detail).mockResolvedValue(
    result({ rows: [rows[0]], total: 1 }),
  );
  showPP();
  const search = await screen.findByLabelText('Buscar sessões');
  fireEvent.change(search, { target: { value: 'duplicado' } });
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar duplicado row-1' }),
  );
  await screen.findByRole('region', { name: 'Detalhes da sessão' });
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  await act(async () =>
    fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' })),
  );
  expect(
    screen.queryByRole('region', { name: 'Detalhes da sessão' }),
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText('Buscar sessões')).toHaveFocus();
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
  fireEvent.click(await screen.findByRole('tab', { name: 'Informações' }));
  expect(
    screen.getByRole('region', { name: 'Detalhes da sessão' }),
  ).toHaveTextContent('row-2');
  await act(async () => first.resolve(result({ rows: [rows[0]], total: 1 })));
  expect(
    screen.getByRole('region', { name: 'Detalhes da sessão' }),
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
  expect(screen.getAllByRole('button', { name: 'Atualizar' })).toHaveLength(1);
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
  await screen.findByRole('region', { name: 'Detalhes da sessão' });
  expect(api.detail).toHaveBeenCalledWith(
    'company',
    'active',
    'device',
    'observation-a',
    'row-2',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
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
