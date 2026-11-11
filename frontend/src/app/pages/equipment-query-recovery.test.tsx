import { device, data } from '../../shared/testing/equipment-test-fixture';
import { findDevicePrimary } from '../test-navigation';
import { openDeviceSheet } from '../test-navigation';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';

import { equipmentAPI as api } from '../../features/equipment/equipment-api';

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
it('an uncertain query start reuses its ID and never retries authentication by itself', async () => {
  const trusted = { ...device, fingerprint: 'SHA256:fixture' };
  vi.mocked(api.list).mockResolvedValue(data([trusted]));
  vi.mocked(api.start)
    .mockResolvedValueOnce({ ...data(), code: 'DEADLINE_EXCEEDED' })
    .mockResolvedValueOnce({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'job',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      },
    });
  render(<Equipment profile="X" activation="a" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo da operação/);
  expect(api.start).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.start).mock.calls[0][3]).toBe(
    vi.mocked(api.start).mock.calls[1][3],
  );
});
const runningJob = {
  id: 'poll-job',
  device_id: device.id,
  kind: 'device.query-start',
  state: 'RUNNING',
  code: 'OK',
};
it.each(['reject', 'reply'])(
  'polling recovers from %s without another start',
  async (kind) => {
    vi.mocked(api.list).mockResolvedValue(
      data([{ ...device, fingerprint: 'SHA256:test' }]),
    );
    vi.mocked(api.start).mockResolvedValue({
      ...data(),
      data: { ...data().data!, job: runningJob },
    } as EquipmentResult);
    if (kind === 'reject')
      vi.mocked(api.status).mockRejectedValueOnce(new Error('transport'));
    else
      vi.mocked(api.status).mockResolvedValueOnce({
        ...data(),
        code: 'SERVICE_UNAVAILABLE',
      });
    vi.mocked(api.status).mockResolvedValueOnce({
      ...data(),
      data: { ...data().data!, job: { ...runningJob, state: 'SUCCEEDED' } },
    } as EquipmentResult);
    render(
      <Equipment profile={'poll-' + kind} activation="surface" connected />,
    );
    await openDeviceSheet('Router X');
    fireEvent.click(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    );
    await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByText(/Consulta SSH em curso/),
      ).not.toBeInTheDocument(),
    );
    expect(api.start).toHaveBeenCalledTimes(1);
  },
);
it('exhausted status recovery unlocks UI and deliberate retry reconciles the same start ID', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.status).mockRejectedValue(new Error('transport'));
  render(<Equipment profile="poll-exhaust" activation="surface" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(3));
  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    ).toBeEnabled(),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.start).mock.calls[0][3]).toBe(
    vi.mocked(api.start).mock.calls[1][3],
  );
});
it('uncertain IDs survive remount and query/inspect/device alternation; interrupted receipt requires deliberate new start', async () => {
  const trusted = { ...device, fingerprint: 'SHA256:test' },
    other = { ...trusted, id: 'other', name: 'Router B' };
  vi.mocked(api.list).mockResolvedValue(data([trusted, other]));
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  const mount = () =>
    render(<Equipment profile="retain" activation="surface" connected />);
  const view = mount();
  await openDeviceSheet('Router X');
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo/);
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await waitFor(() => expect(api.inspect).toHaveBeenCalledTimes(1));
  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    ).toBeEnabled(),
  );
  if (screen.queryByRole('button', { name: 'Voltar' }))
    fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  await openDeviceSheet('Router B');
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  view.unmount();
  mount();
  await openDeviceSheet('Router X');
  vi.mocked(api.start).mockResolvedValueOnce({
    ...data(),
    code: 'JOB_INTERRUPTED',
  });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/interrompida ou expirou/);
  expect(vi.mocked(api.start).mock.calls[2][3]).toBe(
    vi.mocked(api.start).mock.calls[0][3],
  );
  expect(api.start).toHaveBeenCalledTimes(3);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(4));
  expect(vi.mocked(api.start).mock.calls[3][3]).not.toBe(
    vi.mocked(api.start).mock.calls[0][3],
  );
});
it('an uncertain inspection retains its own ID across unmount and another operation kind', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.inspect)
    .mockRejectedValueOnce(new Error('lost response'))
    .mockResolvedValueOnce({ ...data(), code: 'JOB_INTERRUPTED' });
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  const view = render(
    <Equipment profile="retain-inspect" activation="surface" connected />,
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByText(/O serviço está indisponível/);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo da operação/);
  view.unmount();
  render(<Equipment profile="retain-inspect" activation="surface" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByText(/interrompida ou expirou/);
  expect(api.inspect).toHaveBeenCalledTimes(2);
  expect(vi.mocked(api.inspect).mock.calls[0][3]).toBe(
    vi.mocked(api.inspect).mock.calls[1][3],
  );
  expect(api.start).toHaveBeenCalledTimes(1);
});
it('cancel reconciles the running job without another authentication', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.status).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.cancel).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: { ...runningJob, state: 'CANCELLED', code: 'CANCELLED' },
    },
  } as EquipmentResult);
  render(<Equipment profile="cancel-status" activation="surface" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(screen.queryByText(/Consulta SSH em curso/)).not.toBeInTheDocument(),
  );
  expect(api.cancel).toHaveBeenCalledWith(
    'cancel-status',
    'surface',
    'poll-job',
  );
  expect(api.start).toHaveBeenCalledTimes(1);
});
it('Q01 retires abandoned activations, preserves same-activation uncertain IDs and ignores late starts', async () => {
  const current = { ...device, fingerprint: 'SHA256:accepted' };
  vi.mocked(api.list).mockResolvedValue(data([current]));
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  const props = {
    profile: 'Q01',
    activation: 'a0',
    connected: true,
  };
  const view = render(<Equipment {...props} />);
  for (let i = 0; i < 130; i++) {
    view.rerender(<Equipment {...props} activation={'a' + i} />);
    const connect = await findDevicePrimary(view.container, 'Router X');
    expect(connect).toHaveTextContent(/^(Conectar|Atualizar)$/);
    expect(connect.closest('[hidden]')).toBeNull();
    await act(async () => {
      fireEvent.click(connect);
    });
    expect(api.start).toHaveBeenCalledTimes(i + 1);
    expect(connect).toBeEnabled();
  }
  const operation = vi.mocked(api.start).mock.calls.at(-1)![3];
  view.unmount();
  render(<Equipment {...props} activation="a129" />);
  await openDeviceSheet('Router X');
  fireEvent.click(
    screen.getByRole('button', { name: /^(Conectar|Atualizar)$/ }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(131));
  expect(vi.mocked(api.start).mock.calls.at(-1)![3]).toBe(operation);
  expect(api.inspect).not.toHaveBeenCalled();
});
