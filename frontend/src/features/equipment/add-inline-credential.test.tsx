import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import { equipmentAPI } from './equipment-api';
import { pendingEnrollments } from '../access/equipment-receipts';
import type { useEquipmentState } from './equipment-state';
import type { EquipmentResult } from './equipment-api';

it('submits a new credential with the device and clears the password during an uncertain response', async () => {
  pendingEnrollments.clear();
  let resolve!: (r: EquipmentResult) => void;
  const save = vi.spyOn(equipmentAPI, 'save').mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  const s = {
    busy: false,
    vault: true,
    credentials: [],
    credentialDefaults: {},
    blocks: [],
    setCredentialForm: vi.fn(),
    continueEnrollment: vi.fn(),
    perform: async (call: () => Promise<unknown>) => await call(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  render(
    <AddEquipment
      s={s}
      profile="fixture"
      activation="one"
      locked={false}
      back={vi.fn()}
    />,
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'AP fixture' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.31' },
  });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'concentrator' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'reader' },
  });
  fireEvent.change(screen.getByLabelText('Senha'), {
    target: { value: 'synthetic-only' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  expect(save).toHaveBeenCalledOnce();
  const request = save.mock.calls[0][2];
  expect(request).toMatchObject({
    credential_id: '',
    user: 'reader',
    secret: 'synthetic-only',
  });
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  await act(async () =>
    resolve({
      code: 'SERVICE_UNAVAILABLE',
      profile_id: 'fixture',
      activation_id: 'one',
    }),
  );
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Conferir cadastro' }),
    ).toBeVisible(),
  );
  expect(request.secret).toBe('');
  expect(save).toHaveBeenCalledOnce();
  pendingEnrollments.clear();
});

it('does not submit a locked form even through a direct submit event', () => {
  const perform = vi.fn();
  const s = {
    busy: false,
    vault: true,
    credentials: [],
    credentialDefaults: {},
    blocks: [],
    setCredentialForm: vi.fn(),
    perform,
  } as unknown as ReturnType<typeof useEquipmentState>;
  render(
    <AddEquipment
      s={s}
      profile="fixture"
      activation="one"
      locked
      back={vi.fn()}
    />,
  );
  fireEvent.submit(screen.getByLabelText('Nome').closest('form')!);
  expect(perform).not.toHaveBeenCalled();
});

it('reconciles a lost direct credential enrollment before retrying the same IDs', async () => {
  pendingEnrollments.clear();
  const requests: { id: string; operation_id: string; secret: string }[] = [];
  const save = vi
    .spyOn(equipmentAPI, 'save')
    .mockImplementation(async (_p, _a, request) => {
      requests.push({
        id: request.id,
        operation_id: request.operation_id,
        secret: request.secret,
      });
      return {
        code: requests.length === 1 ? 'SERVICE_UNAVAILABLE' : 'OK',
        profile_id: 'retry',
        activation_id: 'one',
      };
    });
  const status = vi.spyOn(equipmentAPI, 'enrollmentStatus').mockResolvedValue({
    code: 'OK',
    profile_id: 'retry',
    activation_id: 'one',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      operation_state: 'NOT_FOUND',
    },
  });
  const s = {
    busy: false,
    vault: true,
    credentials: [],
    credentialDefaults: {},
    blocks: [],
    setCredentialForm: vi.fn(),
    continueEnrollment: vi.fn(),
    reloadCredentials: vi.fn(),
    perform: async (call: () => Promise<unknown>) => await call(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  render(
    <AddEquipment
      s={s}
      profile="retry"
      activation="one"
      locked={false}
      back={vi.fn()}
    />,
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'Router fixture' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.31' },
  });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'concentrator' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'reader' },
  });
  fireEvent.change(screen.getByLabelText('Senha'), {
    target: { value: 'synthetic-only' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Conferir cadastro' }),
  );
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  expect(status).toHaveBeenCalledWith(
    'retry',
    'one',
    requests[0].id,
    requests[0].operation_id,
  );
  expect(save).toHaveBeenCalledOnce();
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  fireEvent.change(screen.getByLabelText('Senha'), {
    target: { value: 'synthetic-only' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(requests[1].id).toBe(requests[0].id);
  expect(requests[1].operation_id).toBe(requests[0].operation_id);
  expect(requests[1].secret).toBe('synthetic-only');
  pendingEnrollments.clear();
});
