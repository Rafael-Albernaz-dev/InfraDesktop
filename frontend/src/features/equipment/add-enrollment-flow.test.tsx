import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import { equipmentAPI } from './equipment-api';
import { pendingEnrollments } from './equipment-receipts';
import type { useEquipmentState } from './equipment-state';

it('keeps an uncertain create receipt across reopening and reconciles without resending', async () => {
  pendingEnrollments.clear();
  const scope = { profile_id: 'synthetic', activation_id: 'one' };
  const save = vi
    .spyOn(equipmentAPI, 'save')
    .mockResolvedValue({ ...scope, code: 'SERVICE_UNAVAILABLE' });
  const status = vi.spyOn(equipmentAPI, 'enrollmentStatus').mockResolvedValue({
    ...scope,
    code: 'OK',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      operation_state: 'NOT_FOUND',
    },
  });
  const perform = async (call: () => Promise<unknown>) => await call();
  const s = {
    busy: false,
    surfaceRef: { current: 0 },
    credentials: [],
    credentialDefaults: {},
    setCredentialForm: vi.fn(),
    perform,
    showSavedDevice: vi.fn(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  const props = {
    s,
    profile: scope.profile_id,
    activation: scope.activation_id,
    locked: false,
    back: vi.fn(),
  };
  const view = render(<AddEquipment {...props} />);
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'AP fixture' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.31' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  expect(save).toHaveBeenCalledOnce();
  const sent = save.mock.calls[0][2];
  view.unmount();
  render(<AddEquipment {...props} />);
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await waitFor(() =>
    expect(status).toHaveBeenCalledWith(
      scope.profile_id,
      scope.activation_id,
      sent.id,
      sent.operation_id,
    ),
  );
  expect(save).toHaveBeenCalledOnce();
  expect(await screen.findByRole('button', { name: 'Salvar' })).toBeEnabled();
  pendingEnrollments.clear();
});
