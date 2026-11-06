import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import type { useEquipmentState } from './equipment-state';
it('reports an invalid IP on the field and does not call the service', async () => {
  const perform = vi.fn();
  const s = {
    busy: false,
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
      locked={false}
      back={vi.fn()}
    />,
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'AP fixture' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '10001' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  const ip = screen.getByLabelText('IP de gerenciamento');
  expect(ip).toHaveAttribute('aria-invalid', 'true');
  expect(ip).toHaveFocus();
  expect(ip).toHaveAccessibleDescription(/IPv4.*10\.0\.0\.1.*IPv6/);
  expect(perform).not.toHaveBeenCalled();
});

it('associates a daemon address rejection with the field without leaving the draft', async () => {
  const s = {
    busy: false,
    credentials: [],
    credentialDefaults: {},
    blocks: [],
    setCredentialForm: vi.fn(),
    perform: vi.fn(async () => ({
      code: 'DEVICE_ADDRESS_INVALID',
      profile_id: 'fixture',
      activation_id: 'one',
    })),
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
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  const ip = screen.getByLabelText('IP de gerenciamento');
  await vi.waitFor(() => expect(ip).toHaveAttribute('aria-invalid', 'true'));
  expect(ip).toHaveFocus();
  expect(screen.getByLabelText('Nome')).toHaveValue('AP fixture');
  expect(ip).toHaveValue('192.0.2.31');
});
