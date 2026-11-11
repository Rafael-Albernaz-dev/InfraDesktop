import { device, data } from '../../shared/testing/equipment-test-fixture';

import { beginAdd, openDeviceSheet } from '../test-navigation';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';

import { equipmentAPI as api } from '../../features/equipment/equipment-api';
import { pppoeReadAPI as readAPI } from '../../platform/wails/presentation';
import type { EquipmentResult } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({
    deviceMonitoringAPI: {
      current: vi.fn().mockResolvedValue({
        code: 'OK',
        profile_id: 'company',
        activation_id: 'active',
        data: { total: 0 },
      }),
    },
  }),
);
beforeEach(resetEquipmentMocks);
import { resetEquipmentMocks } from '../../shared/testing/equipment-setup';
it('first acceptance is deliberate without an invented origin and failed acceptance cannot authenticate', async () => {
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: {
        id: 'j',
        device_id: device.id,
        kind: 'trust.inspect',
        state: 'SUCCEEDED',
        code: 'OK',
        fingerprint: 'SHA256:fixture',
      },
    },
  } as EquipmentResult);
  vi.mocked(api.confirm).mockResolvedValue({
    ...data(),
    code: 'TRUST_CHANGED',
  });
  render(<Equipment profile="X" activation="a" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByRole('dialog', { name: 'Primeira conexão' });
  expect(api.start).not.toHaveBeenCalled();
  expect(
    screen.getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  ).toBeEnabled();
  expect(
    screen.queryByLabelText('Origem da conferência'),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  );
  await waitFor(() => expect(api.confirm).toHaveBeenCalledTimes(1));
  expect(
    await within(screen.getByRole('dialog')).findByRole('alert'),
  ).toHaveTextContent('A chave do destino mudou');
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  expect(api.start).not.toHaveBeenCalled();
});
it.each(['OK', 'TRUST_CHANGED'])(
  'late inspection status %s cannot open a second dialog or place an error on a newer form',
  async (code) => {
    let resolve!: (r: EquipmentResult) => void;
    vi.mocked(api.inspect).mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'late-inspect-job',
          device_id: device.id,
          kind: 'trust.inspect',
          state: 'RUNNING',
          code: 'OK',
        },
      },
    });
    vi.mocked(api.status).mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    render(
      <Equipment profile={'inspect-form-' + code} activation="a" connected />,
    );
    await openDeviceSheet('Router X');
    fireEvent.click(
      screen.getByRole('button', { name: 'Verificar identidade' }),
    );
    await waitFor(() => expect(api.status).toHaveBeenCalledTimes(1));
    await beginAdd();
    await screen.findByRole('heading', { name: 'Adicionar equipamento' });
    await act(async () =>
      resolve({
        ...data(),
        data: {
          ...data().data!,
          job: {
            id: 'late-inspect-job',
            device_id: device.id,
            kind: 'trust.inspect',
            state: code === 'OK' ? 'SUCCEEDED' : 'FAILED',
            code,
            fingerprint: code === 'OK' ? 'SHA256:fixture' : undefined,
          },
        },
      }),
    );
    expect(
      screen.queryByRole('dialog', { name: 'Primeira conexão' }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: 'Adicionar equipamento' }),
    ).toBeVisible();
    expect(
      screen.queryByText(/A chave do destino mudou/),
    ).not.toBeInTheDocument();
    expect(api.confirm).not.toHaveBeenCalled();
    expect(api.start).not.toHaveBeenCalled();
  },
);
it('completed inspection polling on its original surface still offers deliberate trust confirmation', async () => {
  const job = {
    id: 'original-inspect-job',
    device_id: device.id,
    kind: 'trust.inspect',
    state: 'RUNNING',
    code: 'OK',
  };
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job },
  });
  vi.mocked(api.status).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: { ...job, state: 'SUCCEEDED', fingerprint: 'SHA256:fixture' },
    },
  });
  render(<Equipment profile="inspect-original" activation="a" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  const panel = await screen.findByRole('dialog', {
    name: 'Primeira conexão',
  });
  expect(screen.getAllByRole('dialog')).toHaveLength(1);
  expect(within(panel).getByText('SHA256:fixture')).toBeInTheDocument();
  expect(
    within(panel).getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  ).toBeEnabled();
  expect(api.start).not.toHaveBeenCalled();
  expect(api.confirm).not.toHaveBeenCalled();
});
it('concentrator detail exposes one health entry without a saved reading', async () => {
  vi.mocked(readAPI.start).mockResolvedValue({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { total: 0, job: { id: 'health-job', state: 'RUNNING', code: 'OK' } },
  });
  vi.mocked(readAPI.status)
    .mockResolvedValueOnce({
      code: 'OK',
      profile_id: 'X',
      activation_id: 'a',
      data: {
        total: 1,
        job: { id: 'health-job', state: 'SUCCEEDED', code: 'OK' },
        read: {
          scope: 'health',
          pools_available: true,
          uplinks_available: true,
          pool_total: 1,
          uplink_total: 0,
          started_at: '2026-10-27T00:00:00Z',
          ended_at: '2026-10-27T00:00:02Z',
          duration_ns: 2e9,
          rows: [
            {
              kind: 'pool',
              name: 'pool-from-real-composition',
              used: 0,
              size: 3,
              uptime: { known: false, min: 0, max: 0 },
            },
          ],
        },
      },
    })
    .mockResolvedValue({
      code: 'OK',
      profile_id: 'X',
      activation_id: 'a',
      data: {
        total: 0,
        job: { id: 'health-job', state: 'FAILED', code: 'PARTIAL' },
      },
    });
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'pinned' }]),
  );
  render(
    <Equipment
      profile="X"
      activation="a"
      connected

      targetID={device.id}
    />,
  );
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Portas e saúde' }));
  const health = () =>
    within(screen.getByRole('region', { name: 'Pools IPv4' }));
  expect(
    screen.queryByRole('button', { name: 'Medir banda da página' }),
  ).not.toBeInTheDocument();
  fireEvent.click(health().getByRole('button', { name: 'Atualizar' }));
  expect(
    await screen.findByText('pool-from-real-composition'),
  ).toBeInTheDocument();
  expect(readAPI.start).toHaveBeenCalledWith(
    'X',
    'a',
    device.id,
    expect.any(String),
    'health',
    '',
    [],
    device.revision,
  );
  fireEvent.click(health().getByRole('button', { name: 'Atualizar' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Não foi possível concluir esta leitura. Tente novamente.',
  );
  expect(
    screen.queryByText('pool-from-real-composition'),
  ).not.toBeInTheDocument();
});
it('equipment list projects saved health with role and explicit reading age', async () => {
  const at = new Date(Date.now() - 60000).toISOString();
  vi.mocked(api.list).mockResolvedValue(
    data([
      {
        ...device,
        monitoring_at: at,
        monitoring: {
          version: '7.14.3',
          cpu: 37,
          uptime: { known: true, min: 172800e9, max: 172801e9 },
          clock_offset: { known: false, min: 0, max: 0 },
        },
      },
    ]),
  );
  render(<Equipment profile="X" activation="a" connected />);
  const row = await screen.findByRole('row', { name: /Router X/ });
  expect(within(row).getByText('Concentrador')).toBeInTheDocument();
  expect(within(row).getByText('37 %')).toBeInTheDocument();
  expect(within(row).getByText('2 d')).toHaveAttribute(
    'title',
    '172.800–172.801 s (intervalo observado)',
  );
  expect(within(row).getByText('há 1 min')).toBeInTheDocument();
});
