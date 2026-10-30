import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioEquipment, RadioLinkPanel } from './radio';
import { radioAPI as api } from './radio-api';
import type { RadioReading, RadioResult, RadioView } from './radio-api';
import type { Device } from './equipment-api';
vi.mock('./radio-api', () => ({
  radioAPI: {
    current: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    confirmFamily: vi.fn(),
    link: vi.fn(),
    confirmLink: vi.fn(),
  },
}));
vi.mock('./equipment-api', () => ({
  equipmentMessage: (c: string) => c,
  equipmentAPI: { list: vi.fn() },
}));
const device = {
  id: 'ap',
  revision: 3,
  role: 'ap',
  active: true,
  fingerprint: 'pinned',
} as Device;
const response = (data?: Partial<RadioView>): RadioResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, more: false, next_offset: 0, ...data },
});
const reading: RadioReading = {
  id: 'read',
  device_id: 'ap',
  family: 'routeros-wireless-v6',
  version: '6.49.10',
  perspective: 'AP',
  observed_at: '2026-10-29T00:00:00Z',
  compatible: true,
  revision: 3,
  credential_revision: 1,
  connection: 'pinned',
  interfaces: [],
  clients: [
    {
      mac: '02:00:00:00:10:03',
      name: 'CPE-I1',
      interface: 'wlan1',
      bssid: '02:00:00:00:10:05',
      wireless_uptime: '1h',
      last_activity: '90ms',
      metrics: [
        {
          name: 'signal',
          value: -70,
          unit: 'dBm',
          direction: 'CPE → AP',
          perspective: 'AP',
          device_id: 'ap',
          interface: 'wlan1',
          observed_at: '2026-10-29T00:00:00Z',
          quality: 'observed',
        },
      ],
    },
  ],
  metrics: [],
  seconds: 1,
  had_clients: true,
};
const link = {
  cpe_id: 'cpe',
  ap_id: 'ap',
  mac: '02:00:00:00:10:03',
  interface: 'wlan1',
  bssid: '02:00:00:00:10:05',
  evidence: 'MAC na registration table',
  origin: 'sugerido-confirmado',
  concentrator_id: 'concentrator',
  scope: 'pppoe-active-v1',
  login: 'X',
  confirmed_at: '2026-10-29T00:00:00Z',
  confirmed_now: true,
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(api.link).mockResolvedValue(response({ link }));
});
it('opens saved radio without SSH, has one primary query, and preserves absent CCQ with signed pt-BR signal and wireless age', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading,
      total: 1,
      profile: {
        device_id: 'ap',
        family: reading.family,
        confirmed: true,
        warning: 15,
        critical: 20,
      },
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
    />,
  );
  await screen.findByText('-70 dBm');
  expect(api.start).not.toHaveBeenCalled();
  expect(screen.getByText('Visto há 90 ms')).toBeInTheDocument();
  expect(screen.getByText('Uptime wireless: 1 h')).toBeInTheDocument();
  expect(screen.queryByText('CCQ rx')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Consultar rádio' })).toHaveClass(
    'primary',
  );
  expect(
    screen
      .getByRole('region', { name: 'Rádio do equipamento' })
      .querySelectorAll('button.primary'),
  ).toHaveLength(1);
  expect(
    screen.getByText(/Taxa negociada, não é velocidade/),
  ).toBeInTheDocument();
});
it('suggestion needs confirmation before link query; AP result survives an independent CPE failure', async () => {
  vi.mocked(api.link).mockResolvedValue(response({ suggestions: [link] }));
  vi.mocked(api.confirmLink).mockResolvedValue(response({ link }));
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      link,
      reading,
      ap_code: 'OK',
      cpe_code: 'REMOTE_AUTH_FAILED',
      job: { id: 'job', state: 'FAILED', code: 'PARTIAL' },
    }),
  );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="2 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: /Confirmar sugestão/ }),
  );
  await screen.findByRole('button', { name: 'Consultar enlace' });
  expect(api.start).not.toHaveBeenCalled();
  expect(api.confirmLink).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: 'Consultar enlace' }));
  await screen.findByText('-70 dBm');
  expect(screen.getByText('REMOTE_AUTH_FAILED')).toBeInTheDocument();
  expect(
    within(screen.getByRole('region', { name: 'Enlace rádio' })).getByText(
      'Uptime PPPoE: 2 h',
    ),
  ).toBeInTheDocument();
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: 'Live curto' })).toBeDisabled();
});
it('hiding stops the live and returning never renews it; a late start is cancelled on unmount', async () => {
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      cost: {
        samples: Array(10).fill(1),
        enabled: true,
        reason: 'Custo medido.',
        p95_seconds: 1,
        interval_seconds: 5,
      },
    }),
  );
  let finish!: (r: RadioResult) => void;
  vi.mocked(api.start).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve;
    }),
  );
  const view = render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Live curto' }));
  expect(vi.mocked(api.start).mock.calls[0][2].mode).toBe('live');
  view.unmount();
  await act(async () =>
    finish(response({ job: { id: 'late-job', state: 'RUNNING', code: 'OK' } })),
  );
  await waitFor(() => expect(api.cancelOperation).toHaveBeenCalled());
  expect(api.status).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(1);
});
it('visibility loss cancels an active job exactly once and never issues a replacement read', async () => {
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      cost: {
        samples: Array(10).fill(1),
        enabled: true,
        reason: 'Medido',
        p95_seconds: 1,
        interval_seconds: 5,
      },
    }),
  );
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'live-job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      link,
      reading,
      job: { id: 'live-job', state: 'RUNNING', code: 'OK' },
    }),
  );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Live curto' }));
  await waitFor(() => expect(api.status).toHaveBeenCalled());
  Object.defineProperty(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    'visibilityState',
    {
      configurable: true,
      value: 'hidden',
    },
  );
  fireEvent(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    new Event('visibilitychange'),
  );
  await waitFor(() =>
    expect(api.cancel).toHaveBeenCalledWith('company', 'active', 'live-job'),
  );
  Object.defineProperty(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    'visibilityState',
    {
      configurable: true,
      value: 'visible',
    },
  );
  fireEvent(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    new Event('visibilitychange'),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.cancel).toHaveBeenCalledTimes(1);
});
it('R12: Parar followed by a delayed start cannot make the next stop target the old job', async () => {
  let first!: (r: RadioResult) => void, second!: (r: RadioResult) => void;
  vi.mocked(api.start)
    .mockReturnValueOnce(
      new Promise((resolve) => {
        first = resolve;
      }),
    )
    .mockReturnValueOnce(
      new Promise((resolve) => {
        second = resolve;
      }),
    );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar enlace' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Consultar enlace' }),
    ).toBeEnabled(),
  );
  await act(async () =>
    first(
      response({
        job: { id: 'stopped-late-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Consultar enlace' }));
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Consultar enlace' }),
    ).toBeEnabled(),
  );
  expect(api.cancel).not.toHaveBeenCalled();
  expect(api.cancelOperation).toHaveBeenCalledWith(
    'company',
    'active',
    'concentrator',
    vi.mocked(api.start).mock.calls[1][2].operation_id,
  );
  await act(async () =>
    second(
      response({
        job: { id: 'second-stopped-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  expect(api.status).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(2);
});
it('R13: a new virtual AP with zero clients has no inherited alert; its own observed 1-to-0 history does alert', async () => {
  const master = {
    name: 'wlan1',
    bssid: '02:00:00:00:01:01',
    clients: 5,
    had_clients: true,
    metrics: [],
  };
  const virtual = {
    name: 'virtual',
    master: 'wlan1',
    bssid: '02:00:00:00:01:02',
    clients: 0,
    had_clients: false,
    metrics: [],
  };
  vi.mocked(api.current).mockResolvedValue(
    response({ reading: { ...reading, interfaces: [master, virtual] } }),
  );
  const first = render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
    />,
  );
  await screen.findByText('AP virtual · virtual');
  expect(
    screen.queryByText('Clientes zero: este AP já teve clientes.'),
  ).not.toBeInTheDocument();
  first.unmount();
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading: {
        ...reading,
        interfaces: [
          { ...master, clients: 0 },
          { ...virtual, had_clients: true },
        ],
      },
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
    />,
  );
  await waitFor(() =>
    expect(
      screen.getAllByText('Clientes zero: este AP já teve clientes.'),
    ).toHaveLength(2),
  );
});
it('a link read error never claims a missing link; local reopening does not send SSH', async () => {
  vi.mocked(api.link)
    .mockResolvedValueOnce({ ...response(), code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce(response({ link }));
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reabrir vínculo armazenado' }),
  );
  expect(
    screen.queryByText('Nenhum vínculo confirmado para esta sessão.'),
  ).not.toBeInTheDocument();
  await screen.findByRole('button', { name: 'Consultar enlace' });
  expect(api.start).not.toHaveBeenCalled();
  expect(api.link).toHaveBeenCalledTimes(2);
});
it('a saved-radio failure recovers locally without reusing a retired reading ID or sending SSH', async () => {
  vi.mocked(api.current)
    .mockResolvedValueOnce({ ...response(), code: 'OBSERVATION_UNAVAILABLE' })
    .mockResolvedValueOnce(response({ reading }));
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
    />,
  );
  const reopen = await screen.findByRole('button', {
    name: 'Reabrir rádio armazenado',
  });
  expect(
    screen.queryByText(/Nenhuma leitura de rádio salva/),
  ).not.toBeInTheDocument();
  expect(reopen).toHaveClass('primary');
  fireEvent.click(reopen);
  await screen.findByText('-70 dBm');
  expect(api.current).toHaveBeenLastCalledWith(
    'company',
    'active',
    'ap',
    '',
    0,
  );
  expect(api.start).not.toHaveBeenCalled();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

it('shows the first measured refusal and separate command/path times in pt-BR', async () => {
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      cost: {
        samples: Array(10).fill(5),
        enabled: false,
        reason: 'Comando da CPE levou 3,20 s (limite 3 s); live desligado.',
        p95_seconds: 5,
        interval_seconds: 15,
        measurements: Array(10).fill({
          ap: { command_seconds: 0.4, path_seconds: 2 },
          cpe: { command_seconds: 3.2, path_seconds: 3.5 },
          seconds: 5,
        }),
      },
    }),
  );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  expect(
    await screen.findByText(/Comando da CPE levou 3,20 s/),
  ).toHaveTextContent('Intervalo: 15 s · p95 5 s');
  expect(screen.getByText(/Última amostra/)).toHaveTextContent(
    'AP: comando 0,4 s, percurso 2 s; CPE: comando 3,2 s, percurso 3,5 s',
  );
  expect(screen.getByRole('button', { name: 'Live curto' })).toBeDisabled();
});
