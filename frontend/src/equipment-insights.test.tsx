import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { EquipmentInsights } from './equipment-insights';
import { deviceMonitoringAPI, pppoeReadAPI } from './presentation';
import type { Device, PPResult, PPReadRow } from './presentation';
vi.mock('./presentation', () => ({
  deviceMonitoringAPI: { current: vi.fn() },
  pppoeReadAPI: {
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    page: vi.fn(),
  },
}));
it('L06T3: saved equipment has one remote action, FastTrack, negotiated ports, exact history and scoped context', async () => {
  const uptime = { known: false, min: 0, max: 0 },
    at = '2026-10-28T17:00:00Z';
  const respond = (rows: PPReadRow[], scope = 'equipment'): PPResult => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      total: rows?.length || 0,
      read: { scope, started_at: '', ended_at: at, duration_ns: 0, rows },
    },
  });
  vi.mocked(deviceMonitoringAPI.current).mockImplementation(
    async (_p, _a, _d, scope) =>
      respond(
        scope === 'equipment'
          ? [
              {
                kind: 'resource',
                health: { fasttrack_rules: 1, uptime, clock_offset: uptime },
                uptime,
                at,
              },
              { kind: 'restarts', used: 1, size: 2, uptime },
              {
                kind: 'port',
                name: 'X-half',
                port: {
                  running: true,
                  speed_bps: 100000000,
                  full_duplex: false,
                  link_downs: '34',
                },
                uptime,
                at,
              },
              {
                kind: 'port',
                name: 'X-full',
                port: {
                  running: true,
                  speed_bps: 100000000,
                  full_duplex: true,
                },
                uptime,
                at,
              },
              {
                kind: 'port',
                name: 'X-down',
                port: { running: false },
                uptime,
                at,
              },
            ]
          : [
              {
                kind: 'port_change',
                name: 'X-half',
                at,
                before_port: {
                  running: true,
                  speed_bps: 100000000,
                  full_duplex: true,
                  link_downs: '33',
                },
                port: {
                  running: true,
                  speed_bps: 100000000,
                  full_duplex: false,
                  link_downs: '34',
                },
                uptime,
              },
            ],
        scope,
      ),
  );
  const device = {
    id: 'device',
    revision: 2,
    active: true,
    fingerprint: 'pinned',
    name: 'Synthetic',
  } as Device;
  const view = render(
    <EquipmentInsights profile="company" activation="active" device={device} />,
  );
  await screen.findByText('Sem controle de banda: FastTrack ativo');
  expect(screen.getByText('100Mbps-half · atenção')).toHaveAttribute(
    'title',
    '100.000.000 bit/s',
  );
  expect(screen.getByText('100Mbps-full · atenção')).toBeInTheDocument();
  expect(screen.getByText('DOWN')).toBeInTheDocument();
  expect(screen.getByText(/Reinícios 7 d: 1 · 30 d: 2/)).toHaveTextContent(
    'reinícios múltiplos no mesmo intervalo contam como um',
  );
  expect(
    screen.getAllByRole('button', { name: 'Atualizar monitoramento' }),
  ).toHaveLength(1);
  expect(pppoeReadAPI.start).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Histórico de portas' }));
  const history = await screen.findByRole('region', {
    name: 'Histórico de portas salvo',
  });
  await within(history).findByText(
    '100Mbps-full (33 quedas) → 100Mbps-half (34 quedas)',
  );
  expect(pppoeReadAPI.start).not.toHaveBeenCalled();
  vi.mocked(deviceMonitoringAPI.current).mockImplementation(
    () => new Promise(() => {}),
  );
  view.rerender(
    <EquipmentInsights
      profile="other-company"
      activation="other-activation"
      device={device}
    />,
  );
  await waitFor(() =>
    expect(
      screen.queryByText('Sem controle de banda: FastTrack ativo'),
    ).not.toBeInTheDocument(),
  );
  expect(screen.queryByText('X-half')).not.toBeInTheDocument();
});

it('L06T3: pruned history is explicit and a retention marker never becomes a fake port event', async () => {
  vi.resetAllMocks();
  const marker = {
    kind: 'availability',
    name: 'retention',
    value: 'limited',
    uptime: { known: false, min: 0, max: 0 },
  };
  vi.mocked(deviceMonitoringAPI.current).mockImplementation(
    async (_p, _a, _d, scope) => ({
      code: 'OK',
      profile_id: 'company',
      activation_id: 'active',
      data: {
        total: 1,
        read: {
          scope,
          started_at: '',
          ended_at: '',
          duration_ns: 0,
          rows: [marker],
        },
      },
    }),
  );
  render(
    <EquipmentInsights
      profile="company"
      activation="active"
      device={
        {
          id: 'device',
          revision: 2,
          active: true,
          fingerprint: 'pinned',
        } as Device
      }
    />,
  );
  expect(await screen.findByText(/Histórico parcial/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Histórico de portas' }));
  const history = await screen.findByRole('region', {
    name: 'Histórico de portas salvo',
  });
  await within(history).findByText(/registros retidos/);
  expect(within(history).queryByRole('table')).not.toBeInTheDocument();
  expect(within(history).queryByText('retention')).not.toBeInTheDocument();
});
