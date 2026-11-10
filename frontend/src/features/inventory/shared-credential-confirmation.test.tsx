import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { CredentialForm } from '../equipment/equipment-credential-form';
import type { Credential } from '../equipment/equipment-api';
const credential: Credential = {
  id: 'synthetic',
  alias: 'Compartilhada',
  user: 'reader',
  revision: 1,
  links: 3,
};
it('confirms a shared password only on saving; cancel retains the draft and sends nothing', () => {
  const save = vi.fn(async () => undefined);
  render(
    <CredentialForm credential={credential} disabled={false} save={save} />,
  );
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic replacement' },
  });
  expect(
    screen.queryByText(
      'Substituir o acesso usado por 3 equipamentos, inclusive inativos.',
    ),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  const dialog = screen.getByRole('dialog', { name: 'Substituir acesso?' });
  expect(
    within(dialog).getByText(
      'Substituir o acesso usado por 3 equipamentos, inclusive inativos.',
    ),
  ).toBeVisible();
  expect(save).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Cancelar' }));
  expect(screen.getByLabelText('Senha remota')).toHaveValue(
    'synthetic replacement',
  );
  expect(save).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Substituir acesso?' }),
    ).getByRole('button', { name: 'Substituir acesso' }),
  );
  expect(save).toHaveBeenCalledOnce();
});
it('does not ask when keeping the password or when the credential has no links', () => {
  const save = vi.fn(async () => undefined),
    view = render(
      <CredentialForm credential={credential} disabled={false} save={save} />,
    );
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  expect(save).toHaveBeenCalledOnce();
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  view.unmount();
  render(
    <CredentialForm
      credential={{ ...credential, links: 0 }}
      disabled={false}
      save={save}
    />,
  );
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic replacement' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  expect(save).toHaveBeenCalledTimes(2);
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
