import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { EquipmentCredentials } from '../../features/equipment/equipment-credentials';
import type { useEquipmentState } from '../../features/equipment/equipment-state';
import { RadioRoles } from '../../features/equipment/radio-contract';

vi.mock('../../features/equipment/credential-usage', () => ({
  useCredentialUsage: () => ({ a: ['Synthetic A'], b: ['Synthetic B'] }),
}));
vi.mock('../../features/equipment/equipment-credential-form', () => ({
  DefaultCredentialDetails: () => null,
}));

it('filters credentials by user, default role and usage, then clears every filter', () => {
  const s = {
    vault: true,
    credentials: [
      { id: 'a', alias: 'Alpha', user: 'reader-a', links: 1 },
      { id: 'b', alias: 'Beta', user: 'reader-b', links: 1 },
      { id: 'c', alias: 'Gamma', user: 'unlinked', links: 0 },
    ],
    credentialDefaults: { [RadioRoles.ap]: 'a', [RadioRoles.cpe]: 'b' },
    setCredentialForm: vi.fn(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  render(
    <EquipmentCredentials
      s={s}
      profile="company"
      activation="active"
      locked={false}
    />,
  );
  fireEvent.change(screen.getByLabelText('Buscar credencial'), {
    target: { value: 'reader-b' },
  });
  expect(screen.getByText('Beta')).toBeVisible();
  expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  fireEvent.change(screen.getByLabelText('Padrão em'), {
    target: { value: RadioRoles.ap },
  });
  expect(screen.getByText('Alpha')).toBeVisible();
  expect(screen.queryByText('Beta')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  fireEvent.change(screen.getByLabelText('Uso'), {
    target: { value: 'unused' },
  });
  const table = screen.getByRole('table');
  expect(within(table).getByText('Gamma')).toBeVisible();
  expect(within(table).queryByText('Alpha')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  expect(screen.getByText('Alpha')).toBeVisible();
  expect(screen.getByText('Beta')).toBeVisible();
  expect(screen.getByLabelText('Buscar credencial')).toHaveValue('');
});
