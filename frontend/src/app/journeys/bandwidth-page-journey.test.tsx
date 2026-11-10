import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoE } from '../../features/pppoe/pppoe';
import type { Device } from '../../features/equipment/equipment-api';
const device = {
  id: 'synthetic-concentrator',
  name: 'Concentrador sintético',
  active: true,
  revision: 1,
  fingerprint: 'synthetic-pin',
} as Device;
function journey() {
  const rows = [1, 2].map((i) => ({
    row_id: `row-${i}`,
    remote_id: `*${i}`,
    login: `cliente-${i}`,
    address: `192.0.2.${i}`,
    state: 'ONLINE',
    uptime: { known: true, min: 1e9, max: 1e9 },
  }));
  const result = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: { total: rows.length, ...data },
  });
  const status = (state = 'SUCCEEDED', id = 'bandwidth', selected = rows) =>
    result({
      job: { id, state, code: 'OK' },
      ...(state === 'SUCCEEDED'
        ? {
            read: {
              scope: 'bandwidth',
              started_at: '2026-11-04T12:00:00Z',
              ended_at: '2026-11-04T12:00:02Z',
              duration_ns: 2e9,
              rows: selected.map((r) => ({
                ...r,
                kind: 'session',
                download: { min: 2_000_000, max: 2_000_000 },
                upload: { min: 500_000, max: 500_000 },
              })),
            },
          }
        : {}),
    });
  const bindings = {
    GetPPPoECurrent: vi.fn(async () =>
      result({
        current: {
          observation_id: 'saved',
          count: 2,
          provenance: { ended_at: '2026-11-04T12:00:00Z' },
          quality: { status: 'complete' },
        },
      }),
    ),
    ListPPPoEReferences: vi.fn(async () => result({ references: [] })),
    ListPPPoESessions: vi.fn(async () => result({ rows })),
    GetPPPoESession: vi.fn(async () => result({ rows: [rows[0]] })),
    PPPoEContextCounts: vi.fn(async () => result({ groups: [] })),
    GetDeviceMonitoring: vi.fn(async () => result()),
    StartPPPoERead: vi.fn(async (...args: unknown[]) =>
      result({ job: { id: String(args[4]), state: 'RUNNING', code: 'OK' } }),
    ),
    PPPoEReadStatus: vi.fn(async () => status()),
    CancelPPPoERead: vi.fn(async () => result()),
    CancelPPPoEReadOperation: vi.fn(async () => result()),
    StopPPPoETraffic: vi.fn(async () => result()),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={() => {}}
    />,
  );
  return { rows, bindings, status };
}
afterEach(() => {
  cleanup();
  vi.stubGlobal('go', undefined);
});
async function pageAction() {
  return screen.findByRole('button', { name: 'Medir banda da página' });
}
it('mede as linhas da página, mostra progresso e resultado e libera busy', async () => {
  const { bindings, status } = journey();
  let finish!: (r: ReturnType<typeof status>) => void;
  bindings.PPPoEReadStatus.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const button = await pageAction();
  await act(async () => fireEvent.click(button));
  expect(
    await screen.findByText('Aguardando duas amostras do concentrador…'),
  ).toBeVisible();
  expect(button).toBeDisabled();
  expect(bindings.StartPPPoERead).toHaveBeenCalledWith(
    'company',
    'active',
    device.id,
    expect.any(String),
    'bandwidth',
    'saved',
    ['row-1', 'row-2'],
    1,
  );
  await waitFor(() => expect(bindings.PPPoEReadStatus).toHaveBeenCalledOnce(), {
    timeout: 5000,
  });
  await act(async () => finish(status()));
  const result = within(
    screen.getByRole('region', { name: 'Banda por sessão' }),
  );
  expect(await result.findAllByText('2 Mbps')).toHaveLength(2);
  expect(result.getAllByText('500 Kbps')).toHaveLength(2);
  expect(button).toBeEnabled();
  expect(bindings.StartPPPoERead).toHaveBeenCalledOnce();
});
it('cancelar medição encerra o job da página e ignora resultado tardio', async () => {
  const { bindings, status } = journey();
  let finish!: (r: ReturnType<typeof status>) => void;
  bindings.PPPoEReadStatus.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const button = await pageAction();
  fireEvent.click(button);
  await waitFor(() => expect(bindings.PPPoEReadStatus).toHaveBeenCalledOnce(), {
    timeout: 5000,
  });
  fireEvent.click(
    await screen.findByRole('button', { name: 'Cancelar medição' }),
  );
  await waitFor(
    () =>
      expect(bindings.CancelPPPoERead).toHaveBeenCalledWith(
        'company',
        'active',
        'bandwidth',
      ),
    { timeout: 5000 },
  );
  await act(async () => finish(status()));
  expect(button).toBeEnabled();
  expect(screen.queryByText('2 Mbps')).not.toBeInTheDocument();
});
it('Conferir andamento reconcilia o mesmo job sem reenviar a medição', async () => {
  const { bindings, status } = journey();
  bindings.PPPoEReadStatus.mockRejectedValueOnce(
    new Error('synthetic lost status'),
  ).mockResolvedValueOnce(status());
  fireEvent.click(await pageAction());
  fireEvent.click(
    await screen.findByRole('button', { name: 'Conferir andamento' }),
  );
  await screen.findAllByText('2 Mbps');
  expect(bindings.PPPoEReadStatus).toHaveBeenCalledTimes(2);
  expect(bindings.StartPPPoERead).toHaveBeenCalledOnce();
});
it('tela da sessão mede só a linha selecionada e apresenta o resultado', async () => {
  const { bindings, rows, status } = journey();
  bindings.PPPoEReadStatus.mockImplementation(async () =>
    status('SUCCEEDED', 'bandwidth', [rows[0]]),
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Detalhar cliente-1 *1' }),
  );
  const panel = within(
    await screen.findByRole('region', { name: 'Detalhes da sessão' }),
  );
  fireEvent.click(await panel.findByRole('button', { name: 'Medir banda' }));
  await waitFor(() =>
    expect(
      bindings.StartPPPoERead.mock.calls.some(
        (args) =>
          args[4] === 'bandwidth' && JSON.stringify(args[6]) === '["row-1"]',
      ),
    ).toBe(true),
  );
  expect(
    await within(panel.getByRole('region', { name: 'Banda' })).findAllByText(
      '2 Mbps',
    ),
  ).toHaveLength(1);
});
