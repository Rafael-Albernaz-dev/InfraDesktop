import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { CredentialFormDialog } from '../../features/equipment/credential-form-dialog';
import { equipmentAPI as api } from '../../features/equipment/equipment-api';
import type { Credential } from '../../features/equipment/equipment-api';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
const base: Credential = {
  id: 'fixture',
  alias: 'sem-uso',
  user: 'reader',
  revision: 2,
  links: 0,
};
function show(credential: Credential, defaults: Record<string, string> = {}) {
  const perform = vi.fn(
    async (call: () => Promise<{ code: string }>, after?: () => void) => {
      const result = await call();
      if (result.code === 'OK') after?.();
      return result;
    },
  );
  render(
    <CredentialFormDialog
      credentialForm={credential}
      setCredentialForm={vi.fn()}
      error=""
      vault
      ready
      profile="fixture"
      activation="a"
      locked={false}
      defaultBusy={false}
      setDefaultBusy={vi.fn()}
      credentialDefaults={defaults}
      credentials={[credential]}
      defaultContextRef={{ current: 'fixture|a' }}
      reloadCredentials={vi.fn()}
      setCredentialError={vi.fn()}
      perform={perform as never}
    />,
  );
}
it('keeps an existing password when its field is empty', async () => {
  vi.mocked(api.putCredential).mockResolvedValue({
    code: 'OK',
    profile_id: 'fixture',
    activation_id: 'a',
  });
  show(base);
  expect(
    screen.getByText('Deixe vazio para manter a senha atual.'),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  await waitFor(() => expect(api.putCredential).toHaveBeenCalled());
  expect(vi.mocked(api.putCredential).mock.calls.at(-1)?.[2]).toMatchObject({
    id: 'fixture',
    revision: 2,
    secret: '',
  });
});
it.each([
  [{ ...base, links: 2 }, {}, 'Em uso por 2 equipamento(s)'],
  [base, { ap: 'fixture' }, 'Credencial padrão por tipo'],
])(
  'disables deletion for references or defaults',
  (credential, defaults, title) => {
    show(credential, defaults);
    expect(
      screen.getByRole('button', { name: 'Excluir credencial' }),
    ).toBeDisabled();
    expect(
      screen.getByRole('button', { name: 'Excluir credencial' }),
    ).toHaveAttribute('title', title);
  },
);
it('asks for confirmation and uses a stable operation id on retry', async () => {
  vi.mocked(api.deleteCredential).mockResolvedValue({
    code: 'SERVICE_UNAVAILABLE',
    profile_id: 'fixture',
    activation_id: 'a',
  });
  show(base);
  fireEvent.click(screen.getByRole('button', { name: 'Excluir credencial' }));
  const dialog = screen.getByRole('dialog', { name: 'Excluir credencial?' });
  expect(within(dialog).getByRole('heading')).toHaveFocus();
  expect(
    within(dialog).getByText(
      'A credencial sem-uso será removida do cofre. Esta ação não pode ser desfeita.',
    ),
  ).toBeInTheDocument();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Cancelar' }));
  expect(api.deleteCredential).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Excluir credencial' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Excluir credencial?' }),
    ).getByRole('button', { name: 'Excluir credencial' }),
  );
  await waitFor(() => expect(api.deleteCredential).toHaveBeenCalledTimes(1));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Excluir credencial?' }),
    ).getByRole('button', { name: 'Excluir credencial' }),
  );
  await waitFor(() => expect(api.deleteCredential).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.deleteCredential).mock.calls[0][3]).toEqual(
    vi.mocked(api.deleteCredential).mock.calls[1][3],
  );
});
