import {
  RadioCodes,
  RadioFamilies,
  RadioRoles,
} from '../../features/equipment/radio-contract';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioEquipment, RadioLinkPanel } from '../../features/radio/radio';
import { radioAPI as api } from '../../features/equipment/radio-api';
import type {
  RadioReading,
  RadioView,
} from '../../features/equipment/radio-api';
import {
  fixtureAPID,
  device,
  response,
  reading,
  link,
} from '../../shared/testing/radio-test-fixtures';
vi.mock('../../features/equipment/radio-api', async () =>
  (await import('../../shared/testing/radio-api-mock')).radioAPIMock({}),
);
vi.mock('../../features/equipment/equipment-api', async () =>
  (await import('../../shared/testing/equipment-api-mock')).equipmentAPIMock({
    equipmentMessage: (c: string) => c,
  }),
);
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(api.link).mockResolvedValue(response({ link }));
});

it('renders tick failures and explicit rate states without turning absence or reset into zero', async () => {
  prepareTickFailureDisplayResponses(prepareTickFailureDisplayTarget());
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
  fireEvent.click(await screen.findByText('Série por tick (1)'));
  const receipt = screen.getByRole('list', { name: 'Recibo dos ticks' });
  expect(receipt).toHaveTextContent('CPU indisponível');
  expect(receipt).toHaveTextContent('ICMP indisponível');
  expect(receipt).not.toHaveTextContent('keepalive');
  expect(receipt).toHaveTextContent('AP: PARTIAL');
  expect(screen.getByText('primeiro tick: base')).toBeInTheDocument();
  expect(screen.getByText('reiniciado')).toBeInTheDocument();
  expect(screen.queryByText('0 bps')).not.toBeInTheDocument();
  expect(
    screen.getByText('Retransmissões tx').nextElementSibling,
  ).toHaveTextContent('não disponível nesta família');
});

it('opens saved radio without SSH, has one primary query, and preserves absent CCQ with signed pt-BR signal and wireless age', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading,
      total: 1,
      profile: {
        device_id: fixtureAPID,
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
  expect(screen.getByText('CCQ rx').nextElementSibling).toHaveTextContent(
    'não disponível nesta família',
  );
  expect(screen.getByRole('button', { name: 'Atualizar' })).toHaveClass(
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
  await screen.findByText('virtual (AP virtual de wlan1)');
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
    fixtureAPID,
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
  expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeDisabled();
});

it('keeps AP-only live explicit when the CPE firmware has no driver', async () => {
  const note =
    'CPE: firmware sem driver (AC synthetic / WA.v8.7.0), dados vistos pelo AP';
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      reading,
      ...{ cpe_note: note },
      cpe_code: RadioCodes.UNSUPPORTED,
      cost: {
        samples: Array(10).fill(1),
        enabled: true,
        reason: 'Custo medido em dez ticks do AP.',
        p95_seconds: 0.4,
        interval_seconds: 1.2,
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
  expect(await screen.findByText(note)).toBeVisible();
  expect(screen.getByText('-70 dBm')).toBeVisible();
  expect(screen.queryByText('Lado CPE')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Atualizar' })).toHaveClass(
    'primary',
  );
  expect(api.start).not.toHaveBeenCalled();
});

it('does not offer family confirmation for an identified firmware without a driver', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading: {
        ...reading,
        family: RadioFamilies.unsupported,
        version: 'WA.v8.7.0',
        ...{ model: 'AC synthetic' },
        perspective: 'CPE',
        clients: [],
        interfaces: [],
        metrics: [],
      },
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={{ ...device, role: RadioRoles.cpe }}
      blocked={false}
    />,
  );
  await screen.findByText(/Driver ausente \(AC synthetic \/ WA\.v8\.7\.0\)/);
  expect(
    screen.queryByRole('button', { name: 'Confirmar família detectada' }),
  ).not.toBeInTheDocument();
  expect(api.confirmFamily).not.toHaveBeenCalled();
});

it('consults a standalone PTP automatically and shows local-only peer and unavailable power', async () => {
  const station: RadioReading = {
    ...reading,
    perspective: 'CPE',
    model: 'RBLHG-5HPnD',
    mode: 'station-bridge',
    wireless_protocol: '802.11',
    clients: [{ ...reading.clients[0], name: 'LAB-PEER' }],
    metrics: [],
  };
  vi.mocked(api.current).mockResolvedValue(response({ reading: station }));
  vi.mocked(api.start).mockResolvedValue(
    response({
      job: { id: 'job', state: 'RUNNING', code: 'OK' } as NonNullable<
        RadioView['job']
      >,
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={{ ...device, role: RadioRoles.ptp, fingerprint: '' }}
      blocked={false}
    />,
  );
  fireEvent.click(await screen.findByText('Detalhes da leitura'));
  expect(
    await screen.findByText(/visto pelo lado local, não cadastrado/),
  ).toBeVisible();
  expect(screen.getByText(/station-bridge/)).toBeVisible();
  expect(
    screen.getByText(/Tensão: não disponível nesta família/),
  ).toBeVisible();
  expect(
    screen.getByText(/Temperatura: não disponível nesta família/),
  ).toBeVisible();
  expect(screen.queryByLabelText('Família do rádio')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Confirmar família detectada' }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: /^(Conectar|Atualizar)$/ }),
  );
  await waitFor(() =>
    expect(api.start).toHaveBeenCalledWith(
      'company',
      'active',
      expect.objectContaining({
        family: 'auto-avulso',
        mode: 'once',
      }),
    ),
  );
});

it('renders WOM collection scope and does not infer model for Intelbras WOM', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading: {
        ...reading,
        family: RadioFamilies.intelbrasWOM,
        model: '',
      },
      total: 1,
      profile: {
        device_id: fixtureAPID,
        family: RadioFamilies.intelbrasWOM,
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
  const scope = await screen.findByTestId('wom-collection-scope');
  expect(scope).toHaveTextContent('Coletado nesta versão (Intelbras WOM)');
  expect(scope).toHaveTextContent('sinal (dBm)');
  expect(scope).toHaveTextContent('Não coletado nesta versão');
  expect(scope).toHaveTextContent('SSID');
  expect(scope).toHaveTextContent('largura de canal');
  expect(scope).toHaveTextContent('porta Ethernet');
  expect(scope).toHaveTextContent('DHCP/QoS');
  expect(
    screen.getByText(/Modelo não coletado nesta versão \(sem inferir\)/),
  ).toBeInTheDocument();
});

function prepareTickFailureDisplayTarget() {
  const metrics = ['baseline', 'reset', 'unavailable', 'observed'].map(
    (quality, i) => ({
      ...reading.clients[0].metrics[0],
      name: [
        'tx_throughput',
        'rx_throughput',
        'tx_retries_delta',
        'tx_packets_delta',
      ][i],
      quality,
      unit: i < 2 ? 'bps' : 'packets',
      value: i === 3 ? 3 : 0,
    }),
  );
  return { metrics };
}

function prepareTickFailureDisplayResponses(
  context: ReturnType<typeof prepareTickFailureDisplayTarget>,
) {
  const { metrics } = context;
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      reading: {
        ...reading,
        scope: 'link-tick',
        clients: [{ ...reading.clients[0], metrics }],
      },
      cost: {
        samples: [],
        p95_seconds: 0.4,
        interval_seconds: 1.2,
        enabled: false,
        reason: 'AP: PARTIAL',
        series: [
          {
            observed_at: reading.observed_at,
            cpu: null as unknown as number[],
            icmp_rtt_ms: null as unknown as number[],
            ap_command_seconds: 0,
            cpe_command_seconds: 0,
            reason: 'AP: PARTIAL',
          },
        ],
      },
    }),
  );
  return { metrics };
}
