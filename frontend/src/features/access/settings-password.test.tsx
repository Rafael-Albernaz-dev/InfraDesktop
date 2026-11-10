import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { SettingsPassword } from './settings-password';
import type { DesktopProps } from './desktop-model';

function fields() {
  for (const [label, value] of [
    ['Senha atual', 'synthetic current password'],
    ['Nova senha', 'synthetic new password'],
    ['Confirmar nova senha', 'synthetic new password'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
}
it('shows an incorrect current password on the settings screen', () => {
  render(
    <SettingsPassword
      p={
        {
          connected: true,
          busy: false,
          changePassword: vi.fn(),
          passwordError: 'SENHA INCORRETA',
          passwordErrorKind: 'incorrect-password',
        } as unknown as DesktopProps
      }
    />,
  );
  expect(screen.getByRole('alert')).toHaveTextContent('SENHA INCORRETA');
  expect(screen.getByRole('alert')).toHaveClass('inline-error-danger');
});
it('checks confirmation, sends once and clears password fields after success', async () => {
  const change = vi.fn().mockResolvedValue(true);
  render(
    <SettingsPassword
      p={
        {
          connected: true,
          busy: false,
          changePassword: change,
        } as unknown as DesktopProps
      }
    />,
  );
  fields();
  fireEvent.change(screen.getByLabelText('Confirmar nova senha'), {
    target: { value: 'different synthetic password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Alterar senha' }));
  expect(screen.getByRole('alert')).toHaveTextContent(
    'As senhas não conferem.',
  );
  expect(change).not.toHaveBeenCalled();
  fields();
  fireEvent.click(screen.getByRole('button', { name: 'Alterar senha' }));
  await waitFor(() => expect(change).toHaveBeenCalledOnce());
  expect(change).toHaveBeenCalledWith(
    'synthetic current password',
    'synthetic new password',
  );
  expect(screen.getByLabelText('Senha atual')).toHaveValue('');
  expect(screen.getByLabelText('Nova senha')).toHaveValue('');
  expect(screen.getByLabelText('Confirmar nova senha')).toHaveValue('');
  expect(await screen.findByRole('status')).toHaveTextContent(
    'Senha alterada.',
  );
});
