import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import App from '../../app/App';
import { access } from '../../platform/wails/presentation';

const mocked = vi.hoisted(() =>
  Object.fromEntries(
    'status,resetAccount,ackRecovery,getBrand,login'
      .split(',')
      .map((name) => [name, vi.fn()]),
  ),
);
vi.mock('../../platform/wails/presentation', () => ({
  access: mocked,
  equipmentAPI: { list: vi.fn() },
}));
const response = (code = 'OK', recovery = '') => ({
  code,
  recovery,
  request_id: 'synthetic',
  instance_id: 'instance',
  activation_id: '',
  profile_id: '',
  heartbeat: 0,
  setup: false,
  authenticated: false,
  name: '',
  theme: 'light',
  companies: [],
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.status).mockResolvedValue(response());
  vi.mocked(access.ackRecovery).mockResolvedValue(response());
});
async function resetForm() {
  render(<App />);
  fireEvent.click(
    await screen.findByRole('button', { name: 'Recuperar acesso' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Perdi a senha e o código' }),
  );
  expect(
    screen.getByRole('heading', { name: 'Redefinir conta' }),
  ).toBeVisible();
  for (const [label, value] of [
    ['Novo usuário', 'synthetic'],
    ['Nova senha', 'synthetic account password'],
    ['Confirmar nova senha', 'synthetic account password'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
}
it.each([
  ['ADMIN_AUTH_CANCELLED', 'A autorização foi cancelada.'],
  [
    'ADMIN_AUTH_UNAVAILABLE',
    'Não foi possível abrir a autorização de administrador neste computador.',
  ],
])(
  'shows %s clearly without exposing a technical error',
  async (code, text) => {
    vi.mocked(access.resetAccount).mockResolvedValue(response(code));
    await resetForm();
    fireEvent.click(screen.getByRole('button', { name: 'Redefinir conta' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(text);
    expect(access.resetAccount).toHaveBeenCalledWith(
      'synthetic',
      'synthetic account password',
    );
    expect(screen.getByLabelText('Nova senha')).toHaveValue('');
  },
);
it('confirms the new password before requesting authorization and displays the issued code', async () => {
  vi.mocked(access.resetAccount).mockResolvedValue(
    response('OK', 'a'.repeat(64)),
  );
  await resetForm();
  fireEvent.change(screen.getByLabelText('Confirmar nova senha'), {
    target: { value: 'different synthetic password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Redefinir conta' }));
  expect(screen.getByRole('alert')).toHaveTextContent(
    'As senhas não conferem.',
  );
  expect(access.resetAccount).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('Confirmar nova senha'), {
    target: { value: 'synthetic account password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Redefinir conta' }));
  expect(
    await screen.findByRole('heading', { name: 'Guarde seu código' }),
  ).toBeVisible();
  await waitFor(() => expect(access.resetAccount).toHaveBeenCalledOnce());
  fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
  await waitFor(() => expect(access.ackRecovery).toHaveBeenCalledOnce());
  expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeVisible();
});

it('requires an explicit login after an indeterminate reset and never retries automatically', async () => {
  vi.mocked(access.resetAccount).mockResolvedValue(
    response('RESET_OUTCOME_UNKNOWN'),
  );
  vi.mocked(access.login).mockResolvedValue(response('UNAUTHENTICATED'));
  await resetForm();
  fireEvent.click(screen.getByRole('button', { name: 'Redefinir conta' }));
  expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeVisible();
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Não foi possível confirmar a redefinição',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Recuperar acesso' }));
  expect(
    screen.getByRole('button', { name: 'Perdi a senha e o código' }),
  ).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar ao login' }));
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'synthetic' },
  });
  fireEvent.change(screen.getByLabelText('Senha'), {
    target: { value: 'synthetic account password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
  await waitFor(() => expect(access.login).toHaveBeenCalledOnce());
  fireEvent.click(screen.getByRole('button', { name: 'Recuperar acesso' }));
  expect(
    screen.getByRole('button', { name: 'Perdi a senha e o código' }),
  ).toBeEnabled();
  expect(access.resetAccount).toHaveBeenCalledOnce();
});
