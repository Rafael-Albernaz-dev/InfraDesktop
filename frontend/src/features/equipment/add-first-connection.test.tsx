import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import { equipmentAPI } from './equipment-api';
import { pendingEnrollments } from '../access/equipment-receipts';
import type { useEquipmentState } from './equipment-state';

it('returns after a single successful save without starting a connection', async () => {
  pendingEnrollments.clear();
  const result = {
    profile_id: 'fixture',
    activation_id: 'fixture',
    code: 'OK',
  };
  vi.spyOn(equipmentAPI, 'save').mockResolvedValue(result);
  const continueEnrollment = vi.fn();
  const back = vi.fn();
  const s = {
    busy: false,
    credentials: [],
    credentialDefaults: {},
    blocks: [],
    setCredentialForm: vi.fn(),
    reloadCredentials: vi.fn(),
    continueEnrollment,
    perform: async (call: () => Promise<unknown>) => await call(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  render(
    <AddEquipment
      s={s}
      profile="fixture"
      activation="fixture"
      locked={false}
      back={back}
    />,
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'AP fixture' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.31' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  await waitFor(() => expect(back).toHaveBeenCalledOnce());
  expect(continueEnrollment).not.toHaveBeenCalled();
});
