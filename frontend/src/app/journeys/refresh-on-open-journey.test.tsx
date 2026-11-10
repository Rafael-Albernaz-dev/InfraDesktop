import { StrictMode } from 'react';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Equipment } from '../../features/equipment/equipment';
import { RadioEquipment } from '../../features/radio/radio';
import {
  RadioCodes,
  RadioRoles,
} from '../../features/equipment/radio-contract';
import {
  device as radioDevice,
  reading,
} from '../../features/radio/radio-test-fixtures';
import type { Device } from '../../features/equipment/equipment-api';
const at = (seconds: number) =>
  new Date(Date.now() - seconds * 1000).toISOString();
function fixtures() {
  const device = {
    ...radioDevice,
    id: 'refresh-router',
    name: 'Concentrador sintético',
    role: 'concentrator',
    credential_id: 'bound-credential',
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
      observed_at: at(0),
    },
  } as Device;
  const observation = {
    observation_id: 'saved-observation',
    count: 1,
    provenance: { ended_at: at(61) },
    quality: { status: 'complete' },
  };
  const result = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [device],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      total: 1,
      ...data,
    },
  });
  const bindings = {
    ListEquipment: vi.fn(async () => result()),
    ListCredentials: vi.fn(async () => result()),
    GetEquipment: vi.fn(async () => result({ device })),
    GetDeviceMonitoring: vi.fn(async () => result()),
    StartQuery: vi.fn(async () =>
      result({
        job: {
          id: 'query',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'RUNNING',
          code: 'OK',
        },
      }),
    ),
    QueryStatus: vi.fn(async () =>
      result({
        job: {
          id: 'query',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'FAILED',
          code: 'CONNECT_FAILED',
        },
      }),
    ),
    PPPoEContextCounts: vi.fn(async () => result({ groups: [] })),
    GetPPPoECurrent: vi.fn(async () => result({ current: observation })),
    ListPPPoEReferences: vi.fn(async () => result({ references: [] })),
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
    StartPPPoECollection: vi.fn(async () =>
      result({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
    PPPoECollectionStatus: vi.fn(async () =>
      result({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  return { device, observation, bindings, result };
}
afterEach(() => {
  cleanup();
  vi.stubGlobal('go', undefined);
});
function show(device: Device) {
  return render(
    <StrictMode>
      <Equipment
        profile="company"
        activation="active"
        connected

        targetID={device.id}
      />
    </StrictMode>,
  );
}
it('Resumo abre pelo StartQuery e mantém identidade, idade e motivo na falha', async () => {
  const { device, bindings } = fixtures();
  device.result!.observed_at = at(61);
  show(device);
  await waitFor(() => expect(bindings.StartQuery).toHaveBeenCalledOnce());
  expect(bindings.StartQuery.mock.calls[0].slice(0, 3)).toEqual([
    'company',
    'active',
    device.id,
  ]);
  const block = (await screen.findByText('Identidade e sistema')).closest(
    '[data-state]',
  )!;
  expect(block).toHaveAttribute('data-state', 'refreshing');
  expect(
    within(block as HTMLElement).getByText('Resumo preservado'),
  ).toBeVisible();
  await waitFor(() => expect(block).toHaveAttribute('data-state', 'failed'));
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  expect(block).toHaveTextContent('Resumo preservado');
  expect(block).toHaveTextContent(/porta SSH/);
  expect(block).toHaveTextContent(/há/);
  expect(bindings.StartQuery).toHaveBeenCalledOnce();
});
it('Sessões abre pelo StartPPPoECollection e duas aberturas durante a leitura não duplicam o início', async () => {
  const { device, bindings } = fixtures();
  show(device);
  await screen.findByRole('heading', { name: device.name });
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  await waitFor(() =>
    expect(bindings.StartPPPoECollection).toHaveBeenCalledOnce(),
  );
  expect(await screen.findByText('cliente-sintético')).toBeVisible();
  fireEvent.click(screen.getByRole('tab', { name: 'Comparação' }));
  fireEvent.click(screen.getByRole('tab', { name: 'Lista' }));
  await screen.findByText('cliente-sintético');
  expect(bindings.StartPPPoECollection).toHaveBeenCalledOnce();
  expect(bindings.StartQuery).not.toHaveBeenCalled();
});
it.each([RadioRoles.ap, RadioRoles.cpe, RadioRoles.ptp])(
  'Resumo de %s usa StartRadio com a credencial vinculada e não repete após recusa',
  async (role) => {
    const { result } = fixtures();
    const device = {
      ...radioDevice,
      id: `auto-${role}`,
      role,
      credential_id: 'bound-credential',
    };
    const bindings = {
      GetRadio: vi.fn(async () =>
        result({
          reading: { ...reading, device_id: device.id, observed_at: at(61) },
        }),
      ),
      StartRadio: vi.fn<
        (
          profile: string,
          activation: string,
          input: unknown,
        ) => Promise<ReturnType<typeof result>>
      >(async () =>
        result({
          job: {
            id: `refused-${role}`,
            state: 'FAILED',
            code: RadioCodes.REMOTE_AUTH_FAILED,
          },
        }),
      ),
      RadioStatus: vi.fn(async () =>
        result({
          job: {
            id: `refused-${role}`,
            state: 'FAILED',
            code: RadioCodes.REMOTE_AUTH_FAILED,
          },
        }),
      ),
      ListCredentials: vi.fn(async () => result()),
      GetEquipment: vi.fn(async () => result({ device })),
      CancelRadioOperation: vi.fn(async () => result()),
      GetRadioIndexPage: vi.fn(async () => result()),
    };
    vi.stubGlobal('go', { presentation: { App: bindings } });
    const sheet = () => (
      <StrictMode>
        <RadioEquipment
          profile="company"
          activation="active"
          device={device}
          blocked={false}
          refreshAvailable
          tab="Resumo"
        />
      </StrictMode>
    );
    const view = render(sheet());
    await waitFor(() => expect(bindings.StartRadio).toHaveBeenCalledOnce());
    expect(bindings.StartRadio.mock.calls[0][2]).toMatchObject({
      device_id: device.id,
      revision: device.revision,
    });
    await screen.findAllByRole('alert');
    view.unmount();
    render(sheet());
    await screen.findByText('Modelo');
    expect(bindings.StartRadio).toHaveBeenCalledOnce();
  },
);
it('não inicia outra coleta quando a leitura PPPoE está em curso, mesmo com tentativa de Resumo mais recente', async () => {
  const { device, observation, bindings, result } = fixtures();
  device.last_attempt = {
    job_id: 'summary',
    state: 'SUCCEEDED',
    code: 'OK',
    at: at(0),
  } as Device['last_attempt'];
  bindings.GetPPPoECurrent.mockResolvedValue(
    result({
      current: observation,
      attempt: {
        job_id: 'running',
        state: 'RUNNING',
        code: 'OK',
        at: at(30),
        quality: { status: 'complete' },
      },
    }),
  );
  show(device);
  await screen.findByRole('heading', { name: device.name });
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  await screen.findByText('cliente-sintético');
  expect(bindings.StartPPPoECollection).not.toHaveBeenCalled();
});
