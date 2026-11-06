import {
  render,
  screen,
  fireEvent,
  cleanup,
  within,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import { message, recoveryActionMessage } from './microcopy';
import { access, type Result } from './presentation';
import App from './App';

vi.mock('./presentation', () => ({
  access: {
    status: vi.fn(),
    getBrand: vi.fn(),
    saveBrand: vi.fn(),
    renewRecovery: vi.fn(),
    ackRecovery: vi.fn(),
    setTheme: vi.fn(),
    copyRecovery: vi.fn(),
    saveRecovery: vi.fn(),
    selectCompany: vi.fn(),
    logout: vi.fn(),
  },
}));
const state: Result = {
  code: 'OK',
  request_id: 'test',
  instance_id: 'one',
  activation_id: 'a',
  profile_id: 'x',
  heartbeat: 0,
  setup: false,
  authenticated: true,
  name: 'technical',
  theme: 'light',
  recovery: '',
  companies: [
    { id: 'x', name: 'Catalog X' },
    { id: 'y', name: 'Catalog Y' },
  ],
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.status).mockResolvedValue(state);
  vi.mocked(access.selectCompany).mockResolvedValue(state);
  vi.mocked(access.getBrand).mockResolvedValue({
    ...state,
    brand: { name: '', logo: '', accent: 'teal' },
  });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});
it('microcopy preserves operation intent, actions and safe credential ambiguity', () => {
  for (const code of ['UNAUTHENTICATED', 'INVALID_REQUEST'])
    expect(message(code, 'login')).toBe('Usuário ou senha não conferem');
  expect(message('INVALID_REQUEST', 'register')).toContain('8 a 128');
  expect(message('INVALID_REQUEST', 'recover')).toContain('nova senha');
  expect(message('INVALID_REQUEST', 'company')).toContain('empresa');
  expect(message('INVALID_REQUEST', 'company')).not.toContain('senha');
  expect(message('UNAUTHENTICATED', 'renew')).toBe('SENHA INCORRETA');
  expect(message('BUSY', 'company')).toContain('Aguarde');
  expect(message('BUSY', 'company')).not.toContain('Feche');
  expect(message('VERSION_INCOMPATIBLE', 'general')).toContain('Atualize');
  expect(recoveryActionMessage('SAVE_EXISTS', 'save')).toContain('outro nome');
  expect(recoveryActionMessage('CANCELLED', 'save')).toBe('');
  for (const raw of [
    'Error: private /home/user/secret',
    'token=secret',
    'stack trace',
  ])
    expect(message(raw, 'general')).toBe(
      'Não foi possível concluir. Tente novamente.',
    );
});
it('rejecting renewal never suggests a first issuance and security shortcut does not issue', async () => {
  vi.mocked(access.renewRecovery).mockResolvedValue({
    ...state,
    code: 'UNAUTHENTICATED',
  });
  render(<App />);
  fireEvent.click(await screen.findByText('Catalog X'));
  fireEvent.click(await screen.findByRole('button', { name: 'Configurações' }));
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  expect(access.renewRecovery).not.toHaveBeenCalled();
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'wrong' },
    },
  );
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', {
      name: 'Gerar novo código',
    }),
  );
  await screen.findByText('SENHA INCORRETA');
  expect(
    screen.queryByText(/pode ter sido invalidado/),
  ).not.toBeInTheDocument();
  fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
it('a rejected retry does not erase earlier unknown commit', async () => {
  vi.mocked(access.renewRecovery)
    .mockResolvedValueOnce({ ...state, code: 'DEADLINE_EXCEEDED' })
    .mockResolvedValueOnce({ ...state, code: 'UNAUTHENTICATED' });
  render(<App />);
  fireEvent.click(await screen.findByText('Catalog X'));
  fireEvent.click(await screen.findByRole('button', { name: 'Configurações' }));
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'first' },
    },
  );
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', {
      name: 'Gerar novo código',
    }),
  );
  await screen.findByText(/Não recebemos a confirmação/);
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'wrong' },
    },
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Gerar outro código novo' }),
  );
  await screen.findByText(/SENHA INCORRETA/);
  expect(screen.getByText(/Não recebemos a confirmação/)).toBeVisible();
});

it('account dropdown keyboard and dialog trap include readonly textarea', async () => {
  vi.mocked(access.renewRecovery).mockResolvedValue({
    ...state,
    recovery: 'a'.repeat(64),
  });
  render(<App />);
  fireEvent.click(await screen.findByText('Catalog X'));
  const trigger = await screen.findByRole('button', { name: /Conta:/ });
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute('aria-expanded', 'true');
  fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Tema escuro' }), {
    key: 'Escape',
  });
  expect(trigger).toHaveFocus();
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'password' },
    },
  );
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', {
      name: 'Gerar novo código',
    }),
  );
  const code = await screen.findByLabelText('Novo código de recuperação');
  code.focus();
  fireEvent.keyDown(code, { key: 'Tab' });
  expect(screen.getByRole('dialog')).toBeInTheDocument();
  const ack = screen.getByRole('button', { name: 'Guardei meu código' });
  ack.focus();
  fireEvent.keyDown(ack, { key: 'Tab' });
  expect(screen.getByLabelText('Novo código de recuperação')).toHaveFocus();
});

it('profile navigation has an explicit Escape dismissal that returns focus', async () => {
  render(<App />);
  fireEvent.click(await screen.findByText('Catalog X'));
  const trigger = await screen.findByRole('button', { name: /Conta:/ });
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute('aria-expanded', 'true');
  fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Sair da conta' }), {
    key: 'Escape',
  });
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  expect(trigger).toHaveFocus();
});
