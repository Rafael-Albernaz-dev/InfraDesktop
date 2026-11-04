import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { RadioEquipment } from './radio';
import { RadioClientRow } from './radio-client-row';
import { RadioCodes, RadioRoles } from './radio-contract';
import { device as radioDevice, reading } from './radio-test-fixtures';
import { shouldRefreshOnOpen } from './use-refresh-on-open';
import type { Device } from './equipment-api';
import type { RadioClient } from './radio-api';

const at = (seconds: number) =>
  new Date(Date.now() - seconds * 1000).toISOString();
const result = (data = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: {
    devices: [],
    credentials: [],
    next_cursor: 0,
    vault_available: true,
    total: 1,
    ...data,
  },
});
afterEach(() => vi.stubGlobal('go', undefined));

function concentrator(age: number) {
  const device = {
    ...radioDevice,
    id: 'cross-router',
    name: 'Concentrador sintético',
    role: 'concentrator',
    credential_id: 'bound',
    result: {
      identity: 'Resumo preservado',
      family: 'v6',
      source: 'SSH',
      quality: 'complete',
      free_memory_bytes: 1,
      total_memory_bytes: 2,
      board: 'Synthetic',
      version: '6.49',
      uptime: '1h',
      cpu_percent: 5,
      interfaces: [],
      observed_at: at(age),
    },
  } as Device;
  const current = {
    observation_id: 'saved',
    count: 1,
    provenance: { ended_at: at(0) },
    quality: { status: 'complete' },
  };
  const running = {
    id: 'query',
    device_id: device.id,
    kind: 'device.query-start',
    state: 'RUNNING',
    code: 'OK',
  };
  const bindings = {
    ListEquipment: vi.fn(async () => result({ devices: [device] })),
    ListCredentials: vi.fn(async () => result()),
    GetEquipment: vi.fn(async () => result({ device })),
    GetDeviceMonitoring: vi.fn(async () => result()),
    StartQuery: vi.fn(async () => result({ job: running })),
    QueryStatus: vi.fn(async () => result({ job: running })),
    PPPoEContextCounts: vi.fn(async () => result({ groups: [] })),
    GetPPPoECurrent: vi.fn(async () => result({ current })),
    ListPPPoEReferences: vi.fn(async () =>
      result({
        references: [
          {
            reference_id: 'r',
            observation_id: 'base',
            created_at: at(9),
            count: 1,
          },
        ],
      }),
    ),
    ListPPPoESessions: vi.fn(async () =>
      result({
        rows: [
          {
            row_id: 'row',
            login: 'cliente-sintético',
            address: '192.0.2.10',
            state: 'ONLINE',
            uptime: { known: false },
          },
        ],
      }),
    ),
    ComparePPPoE: vi.fn<
      (...args: unknown[]) => Promise<ReturnType<typeof result>>
    >(async () => result({ deltas: [], counts: {}, total: 0 })),
    StartPPPoECollection: vi.fn(async () =>
      result({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
    PPPoECollectionStatus: vi.fn(async () =>
      result({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  render(
    <Equipment
      profile="company"
      activation="active"
      connected
      createRequest={0}
      targetID={device.id}
    />,
  );
  return { device, current, bindings };
}

it('recusa gravada no equipamento sobrevive a reiniciar: AP não tenta sozinho', async () => {
  const device = {
    ...radioDevice,
    id: 'persisted-refusal',
    role: RadioRoles.ap,
    credential_id: 'bound',
    last_attempt: {
      job_id: 'old',
      state: 'FAILED',
      code: RadioCodes.REMOTE_AUTH_FAILED,
      at: at(3600),
    },
  } as Device;
  const bindings = {
    GetRadio: vi.fn(async () =>
      result({
        reading: { ...reading, device_id: device.id, observed_at: at(3600) },
      }),
    ),
    StartRadio: vi.fn(async () => result()),
    ListCredentials: vi.fn(async () => result()),
    GetEquipment: vi.fn(async () => result({ device })),
    GetRadioIndexPage: vi.fn(async () => result()),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
      refreshAvailable
      tab="Resumo"
    />,
  );
  await screen.findByText('Modelo');
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(bindings.StartRadio).not.toHaveBeenCalled();
});

it('consulta em curso: abrir Sessões não dispara coleta nem erro falso', async () => {
  const { bindings } = concentrator(61);
  await waitFor(() => expect(bindings.StartQuery).toHaveBeenCalledOnce());
  fireEvent.click(await screen.findByRole('tab', { name: 'Sessões' }));
  await screen.findByText('cliente-sintético');
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(bindings.StartPPPoECollection).not.toHaveBeenCalled();
  expect(screen.queryByText(/Duas operações/)).toBeNull();
});

it('coleta em curso: voltar ao Resumo não dispara consulta', async () => {
  const { device, current, bindings } = concentrator(0);
  fireEvent.click(await screen.findByRole('tab', { name: 'Sessões' }));
  await waitFor(() => expect(bindings.GetPPPoECurrent).toHaveBeenCalled());
  device.result!.observed_at = at(61);
  bindings.GetPPPoECurrent.mockResolvedValue(
    result({
      current,
      attempt: { state: 'RUNNING', code: 'OK', at: at(1) },
    }) as never,
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Resumo' }));
  await screen.findByText('Identidade e sistema');
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(bindings.StartQuery).not.toHaveBeenCalled();
});

it('Comparação não herda o filtro Estado da Lista', async () => {
  const { bindings } = concentrator(0);
  fireEvent.click(await screen.findByRole('tab', { name: 'Sessões' }));
  await screen.findByText('cliente-sintético');
  const filter = screen
    .getAllByLabelText('Estado')
    .find((e) => !(e as HTMLSelectElement).disabled)!;
  fireEvent.change(filter, { target: { value: 'OFFLINE' } });
  await waitFor(() =>
    expect(bindings.ListPPPoESessions.mock.lastCall).toEqual(
      expect.arrayContaining([expect.objectContaining({ state: 'OFFLINE' })]),
    ),
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  const ref = (await screen.findByLabelText(
    'Referência para comparar',
  )) as HTMLSelectElement;
  fireEvent.change(ref, { target: { value: ref.options[1].value } });
  await waitFor(() => expect(bindings.ComparePPPoE).toHaveBeenCalled());
  expect(bindings.ComparePPPoE.mock.lastCall?.[5]).toMatchObject({ state: '' });
});

it('CCQ do cliente do AP usa a métrica real do parser', () => {
  const metric = {
    name: 'ccq_tx',
    value: 92,
    unit: '%',
    direction: '',
    perspective: 'AP',
    device_id: 'd',
    interface: 'wlan1',
    observed_at: at(0),
    quality: 'local',
  };
  const client: RadioClient = {
    mac: '02:00:5E:00:00:01',
    name: 'CPE',
    interface: 'wlan1',
    bssid: '02:00:5E:00:00:02',
    metrics: [metric],
  };
  render(
    <table>
      <tbody>
        <RadioClientRow client={client} name="CPE" />
      </tbody>
    </table>,
  );
  expect(screen.getByText('92%')).toBeVisible();
});

it('data de falha inválida nunca libera nova tentativa', () => {
  expect(
    shouldRefreshOnOpen({
      now: Date.now(),
      observedAt: at(3600),
      device: {
        active: true,
        fingerprint: 'synthetic',
        credential_id: 'bound',
        last_attempt: { state: 'FAILED', code: 'CONNECT_FAILED', at: 'x' },
      },
      vaultAvailable: true,
      serviceAvailable: true,
      reading: false,
    }),
  ).toBe(false);
});
