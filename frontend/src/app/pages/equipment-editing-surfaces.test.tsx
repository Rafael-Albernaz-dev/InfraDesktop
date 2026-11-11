import { device, data } from '../../shared/testing/equipment-test-fixture';

import { openDeviceSheet } from '../test-navigation';
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
for (const active of [true, false]) {
  for (const fingerprint of ['', 'SHA256:accepted']) {
    it(`saves an edited active=${active} pinned=${Boolean(fingerprint)} device without remote work`, async () => {
      const current = { ...device, active, fingerprint };
      vi.mocked(api.list).mockResolvedValue(data([current]));
      vi.mocked(api.save).mockResolvedValue({
        ...data(),
        data: {
          ...data().data!,
          device: { ...current, name: 'Edited name', revision: 2 },
        },
      });
      render(<Equipment profile="X" activation="a" connected />);
      await openDeviceSheet('Router X');
      fireEvent.click(
        screen.getByRole('button', { name: 'Editar equipamento' }),
      );
      fireEvent.change(screen.getByLabelText('Nome'), {
        target: { value: 'Edited name' },
      });
      fireEvent.click(
        screen.getByRole('button', { name: 'Salvar equipamento' }),
      );
      await waitFor(() =>
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
      );
      expect(api.save).toHaveBeenCalledExactlyOnceWith(
        'X',
        'a',
        expect.objectContaining({
          id: device.id,
          revision: 1,
          name: 'Edited name',
          credential_id: 'credential',
          address: '127.0.0.1',
        }),
      );
      await act(async () => {});
      expect(api.inspect).not.toHaveBeenCalled();
      expect(api.start).not.toHaveBeenCalled();
      expect(api.active).not.toHaveBeenCalled();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  }
}
for (const changeSurface of [true, false]) {
  it(`keeps a late post-query failure on its original surface: changed=${changeSurface}`, async () => {
    let reject!: (reason: Error) => void;
    vi.mocked(api.get).mockImplementation(
      () =>
        new Promise((_resolve, fail) => {
          reject = fail;
        }),
    );
    const current = { ...device, fingerprint: 'SHA256:accepted' };
    vi.mocked(api.list).mockResolvedValue(data([current]));
    vi.mocked(api.start).mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'late-query',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      },
    });
    render(<Equipment profile="late" activation="a" connected />);
    await openDeviceSheet('Router X');
    fireEvent.click(
      screen.getByRole('button', { name: /^(Conectar|Atualizar)$/ }),
    );
    await waitFor(() => expect(api.get).toHaveBeenCalled());
    if (changeSurface) {
      fireEvent.click(
        screen.getByRole('button', { name: 'Editar equipamento' }),
      );
      fireEvent.change(screen.getByLabelText('Nome'), {
        target: { value: 'New draft' },
      });
    }
    await act(async () => reject(new Error('late transport failure')));
    if (changeSurface) {
      expect(screen.getByLabelText('Nome')).toHaveValue('New draft');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(
        screen.getByRole('dialog', { name: 'Editar equipamento' }),
      ).toBeVisible();
    } else
      expect(await screen.findByRole('alert')).toHaveTextContent('serviço');
  });
}
for (const path of ['target', 'detail']) {
  for (const outcome of ['success', 'reject']) {
    for (const changed of [true, false]) {
      it(`scopes ${path} read ${outcome} to its surface changed=${changed}`, async () => {
        let complete!: (r: EquipmentResult) => void;
        let reject!: (e: Error) => void;
        const updated = {
          ...data(),
          data: {
            ...data().data!,
            device: { ...device, name: 'Remote changed' },
          },
        };
        vi.mocked(api.get)
          .mockResolvedValue(updated)
          .mockImplementationOnce(
            () =>
              new Promise((resolve, fail) => {
                complete = resolve;
                reject = fail;
              }),
          );
        const props = {
          profile: 'X',
          activation: 'a',
          connected: true,
          targetID: path === 'target' ? device.id : undefined,
        };
        const view = render(<Equipment {...props} />);
        if (path === 'detail')
          fireEvent.click(await screen.findByRole('row', { name: /Router X/ }));
        await waitFor(() => expect(api.get).toHaveBeenCalledTimes(1));
        if (changed) {
          view.rerender(<Equipment {...props} addPage />);
          fireEvent.change(screen.getByLabelText('Nome'), {
            target: { value: 'Preserved draft' },
          });
        }
        await act(async () => {
          if (outcome === 'reject') reject(new Error('late read'));
          else complete(updated);
        });
        if (changed) {
          expect(screen.getByLabelText('Nome')).toHaveValue('Preserved draft');
          expect(screen.queryByRole('alert')).not.toBeInTheDocument();
          expect(
            screen.queryByRole('heading', {
              name: 'Remote changed',
              hidden: true,
            }),
          ).not.toBeInTheDocument();
        } else if (outcome === 'reject')
          expect(await screen.findByRole('alert')).toHaveTextContent('serviço');
        else
          expect(
            await screen.findByRole('heading', { name: 'Remote changed' }),
          ).toBeVisible();
      });
    }
  }
}
it('persists a real form request with stable operation ID and explicit company context', async () => {
  vi.mocked(api.save)
    .mockResolvedValueOnce({
      ...data(),
      code: 'REVISION_CONFLICT',
    })
    .mockResolvedValueOnce(data());
  render(<Equipment profile="X" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'New destination' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '127.0.0.1' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: 'credential' },
  });
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
  const first = vi.mocked(api.save).mock.calls[0];
  expect(first[0]).toBe('X');
  expect(first[1]).toBe('a');
  expect(first[2].transport).toBe('ssh');
  expect(first[2].operation_id).toMatch(/^[a-f0-9-]{36}$/);
  await screen.findAllByText(/O cadastro mudou/);
  expect(
    screen.queryByRole('button', { name: /^Salvar$/ }),
  ).not.toBeInTheDocument();
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...data(),
    data: { ...data().data!, operation_state: 'NOT_FOUND' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.save).mock.calls[1][2].operation_id).toBe(
    first[2].operation_id,
  );
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
});
it('leaving the Add page does not reopen it', async () => {
  const view = render(
    <Equipment profile="X" activation="a" connected addPage />,
  );
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  view.rerender(<Equipment profile="X" activation="a" connected />);
  await waitFor(() =>
    expect(
      screen.queryByRole('heading', { name: 'Adicionar equipamento' }),
    ).not.toBeInTheDocument(),
  );
});

it.each(['OK', 'REVISION_CONFLICT'])(
  'late %s save cannot close or mark a reopened dialog',
  async (code) => {
    let resolve!: (r: EquipmentResult) => void;
    vi.mocked(api.save).mockImplementationOnce(
      () => new Promise((r) => (resolve = r)),
    );
    const view = render(
      <Equipment profile="late" activation="surface" connected />,
    );
    await openDeviceSheet('Router X');
    fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
    fireEvent.click(
      screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
    );
    await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
    expect(
      screen.getByRole('dialog', { name: 'Editar equipamento' }),
    ).toBeInTheDocument();
    view.unmount();
    render(<Equipment profile="late" activation="surface" connected />);
    await openDeviceSheet('Router X');
    fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
    await act(async () => resolve({ ...data(), code }));
    expect(
      screen.getByRole('dialog', { name: 'Editar equipamento' }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Nome')).toHaveValue('Router X');
    expect(screen.queryByText(/O cadastro mudou/)).not.toBeInTheDocument();
    if (code === 'OK')
      await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  },
);
it('an uncertain enrollment never lends its operation ID to an unrelated edit', async () => {
  const sent: Record<string, unknown>[] = [];
  vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
    sent.push(structuredClone(d));
    return { ...data(), code: 'SERVICE_UNAVAILABLE' };
  });
  render(<Equipment profile="pending-edit" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  for (const [label, value] of [
    ['Nome', 'Uncertain'],
    ['IP de gerenciamento', '127.0.0.1'],
    ['Usuário', 'reader'],
    ['Senha', 'fixture only'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
  expect(screen.queryByLabelText('Senha')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Salvar equipamento' }));
  await waitFor(() => expect(sent).toHaveLength(2));
  expect(sent[1].operation_id).not.toBe(sent[0].operation_id);
  expect(sent[1]).toMatchObject({
    id: device.id,
    revision: 1,
    credential_id: device.credential_id,
    secret: '',
    user: '',
  });
});
