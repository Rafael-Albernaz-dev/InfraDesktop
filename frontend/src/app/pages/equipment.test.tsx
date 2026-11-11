import { device, data } from '../../shared/testing/equipment-test-fixture';

import { openDeviceSheet } from '../test-navigation';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
it.each(['cpe', 'ap', 'concentrator'])(
  'queued pin confirmation routes %s without a foreign collector',
  async (role) => {
    const target = { ...device, role, fingerprint: '' };
    vi.mocked(api.list).mockResolvedValue(data([target]));
    vi.mocked(api.get).mockResolvedValue({
      ...data([target]),
      data: { ...data([target]).data!, device: target },
    });
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
      data: {
        ...data().data!,
        device: { ...target, fingerprint: 'SHA256:fixture', revision: 2 },
      },
    });
    render(<Equipment profile="X" activation="a" connected />);
    await openDeviceSheet('Router X');
    fireEvent.click(
      screen.getByRole('button', { name: 'Verificar identidade' }),
    );
    if (role !== 'concentrator') {
      vi.mocked(api.get).mockResolvedValue({
        ...data([target]),
        data: {
          ...data([target]).data!,
          device: { ...target, fingerprint: 'SHA256:fixture', revision: 2 },
        },
      });
      await waitFor(() => expect(api.inspect).toHaveBeenCalledTimes(1));
      await waitFor(() => expect(api.get).toHaveBeenCalled());
      expect(
        screen.queryByRole('dialog', { name: 'Primeira conexão' }),
      ).not.toBeInTheDocument();
      expect(api.confirm).not.toHaveBeenCalled();
      expect(api.start).not.toHaveBeenCalled();
      return;
    }
    await screen.findByRole('dialog', { name: 'Primeira conexão' });
    fireEvent.click(
      screen.getByRole('button', {
        name: 'Confiar neste equipamento e conectar',
      }),
    );
    await waitFor(() => expect(api.confirm).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(
        screen.queryByRole('dialog', { name: 'Primeira conexão' }),
      ).not.toBeInTheDocument(),
    );
    if (role === 'concentrator')
      await waitFor(() => expect(api.start).toHaveBeenCalledTimes(1));
    else expect(api.start).not.toHaveBeenCalled();
  },
);

it('radio enrollment uses the chosen class default and allows an explicit per-device override', async () => {
  const credentials = [
    {
      id: 'ap-access',
      alias: 'Acesso AP',
      user: 'reader',
      revision: 1,
      links: 0,
    },
    {
      id: 'cpe-access',
      alias: 'Acesso CPE',
      user: 'reader',
      revision: 1,
      links: 0,
    },
    {
      id: 'ptp-access',
      alias: 'Acesso PTP',
      user: 'reader',
      revision: 1,
      links: 0,
    },
  ];
  vi.mocked(api.credentials).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      credentials,
      credential_defaults: {
        ap: 'ap-access',
        cpe: 'cpe-access',
        ptp: 'ptp-access',
      },
    },
  });
  render(<Equipment profile="X" activation="a" connected addPage />);
  const role = await screen.findByRole('combobox', {
    name: 'Papel do equipamento',
  });
  await waitFor(() => expect(api.credentials).toHaveBeenCalled());
  for (const [value, expected] of [
    ['ap', 'ap-access'],
    ['cpe', 'cpe-access'],
    ['ptp', 'ptp-access'],
  ]) {
    fireEvent.change(role, { target: { value } });
    expect(
      screen.getByRole('combobox', { name: 'Credencial de consulta' }),
    ).toHaveValue(expected);
    expect(
      screen.queryByLabelText('Senha', { exact: true }),
    ).not.toBeInTheDocument();
  }
  fireEvent.change(
    screen.getByRole('combobox', { name: 'Credencial de consulta' }),
    { target: { value: 'cpe-access' } },
  );
  expect(
    screen.getByRole('combobox', { name: 'Credencial de consulta' }),
  ).toHaveValue('cpe-access');
  expect(api.start).not.toHaveBeenCalled();
  expect(api.inspect).not.toHaveBeenCalled();
  expect(api.save).not.toHaveBeenCalled();
});
