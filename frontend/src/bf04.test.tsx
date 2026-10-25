import {
  render,
  screen,
  fireEvent,
  act,
  cleanup,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import { message, recoveryActionMessage } from './microcopy';
import { access, type Result } from './presentation';
import App from './App';
import { Appearance } from './appearance';
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
  expect(message('INVALID_REQUEST', 'register')).toContain('15 a 128');
  expect(message('INVALID_REQUEST', 'recover')).toContain('nova senha');
  expect(message('INVALID_REQUEST', 'company')).toContain('empresa');
  expect(message('INVALID_REQUEST', 'company')).not.toContain('senha');
  expect(message('UNAUTHENTICATED', 'renew')).toBe(
    'A senha não confere. Tente novamente.',
  );
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
  fireEvent.click(await screen.findByRole('button', { name: /Conta:/ }));
  fireEvent.click(screen.getByRole('button', { name: 'Segurança' }));
  expect(access.renewRecovery).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('Senha atual'), {
    target: { value: 'wrong' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  await screen.findByText('A senha não confere. Tente novamente.');
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
  fireEvent.click(await screen.findByRole('button', { name: /Conta:/ }));
  fireEvent.click(screen.getByRole('button', { name: 'Segurança' }));
  fireEvent.change(screen.getByLabelText('Senha atual'), {
    target: { value: 'first' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  await screen.findByText(/Não recebemos a confirmação/);
  fireEvent.change(screen.getByLabelText('Senha atual'), {
    target: { value: 'wrong' },
  });
  fireEvent.click(
    screen.getByRole('button', { name: 'Gerar outro código novo' }),
  );
  await screen.findByText(/A senha não confere/);
  expect(screen.getByText(/Não recebemos a confirmação/)).toBeVisible();
});
it('failed brand save keeps draft, successful save updates shell without renaming catalog', async () => {
  vi.mocked(access.saveBrand)
    .mockResolvedValueOnce({ ...state, code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce({
      ...state,
      brand: { name: 'Visual X', logo: '', accent: 'rose' },
    });
  render(<App />);
  fireEvent.click(
    await screen.findByRole('button', { name: 'Abrir configurações' }),
  );
  await act(async () => {});
  fireEvent.change(screen.getByLabelText('Nome visual'), {
    target: { value: 'Visual X' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Rosa' }));
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  await screen.findByRole('alert');
  expect(screen.getByLabelText('Nome visual')).toHaveValue('Visual X');
  expect(screen.getByLabelText('Selecionar empresa')).toHaveTextContent(
    'Catalog X',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  await act(async () => {});
  expect(screen.getByLabelText('Selecionar empresa')).toHaveTextContent(
    'Visual X',
  );
  expect(access.saveBrand).toHaveBeenLastCalledWith('x', 'a', {
    name: 'Visual X',
    logo: '',
    accent: 'rose',
  });
  fireEvent.click(screen.getByLabelText('Selecionar empresa'));
  expect(
    screen.getByRole('row', { name: 'Selecionar Catalog X' }),
  ).toBeInTheDocument();
});
it('loading and failed brand fetch have explicit retry and cannot discard an edited draft', async () => {
  vi.mocked(access.getBrand)
    .mockResolvedValueOnce({ ...state, code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce({
      ...state,
      brand: { name: 'Loaded', logo: '', accent: 'blue' },
    });
  render(<App />);
  fireEvent.click(
    await screen.findByRole('button', { name: 'Abrir configurações' }),
  );
  await screen.findByRole('alert');
  expect(screen.getByLabelText('Nome visual')).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
  await act(async () => {});
  expect(screen.getByLabelText('Nome visual')).toHaveValue('Loaded');
  expect(screen.getByLabelText('Nome visual')).toBeEnabled();
});
it('account dropdown keyboard and dialog trap include readonly textarea', async () => {
  vi.mocked(access.renewRecovery).mockResolvedValue({
    ...state,
    recovery: 'a'.repeat(64),
  });
  render(<App />);
  const trigger = await screen.findByRole('button', { name: /Conta:/ });
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute('aria-expanded', 'true');
  fireEvent.keyDown(screen.getByRole('button', { name: 'Segurança' }), {
    key: 'Escape',
  });
  expect(trigger).toHaveFocus();
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole('button', { name: 'Segurança' }));
  fireEvent.change(screen.getByLabelText('Senha atual'), {
    target: { value: 'password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
  const code = await screen.findByLabelText('Novo código de recuperação');
  code.focus();
  fireEvent.keyDown(code, { key: 'Tab' });
  expect(screen.getByRole('dialog')).toBeInTheDocument();
  const ack = screen.getByRole('button', { name: 'Guardei meu código' });
  ack.focus();
  fireEvent.keyDown(ack, { key: 'Tab' });
  expect(screen.getByRole('button', { name: 'Fechar painel' })).toHaveFocus();
});
it('brand mutation waits while save is pending and preview remains confirmed', () => {
  render(
    <Appearance
      brand={{ name: 'Saved', logo: '', accent: 'teal' }}
      theme="light"
      setTheme={() => {}}
      busy
      saveBrand={async () => true}
    />,
  );
  expect(screen.getByLabelText('Nome visual')).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Restaurar padrão' }),
  ).toBeDisabled();
  expect(screen.getByText('Saved')).toBeInTheDocument();
});
it('reconciles a committed brand after navigation discarded the original response', async () => {
  let resolve!: (result: Result) => void;
  vi.mocked(access.saveBrand).mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  vi.mocked(access.getBrand)
    .mockResolvedValueOnce({
      ...state,
      brand: { name: '', logo: '', accent: 'teal' },
    })
    .mockResolvedValue({
      ...state,
      brand: { name: 'Saved later', logo: '', accent: 'blue' },
    });
  render(<App />);
  fireEvent.click(
    await screen.findByRole('button', { name: 'Abrir configurações' }),
  );
  await act(async () => {});
  fireEvent.change(screen.getByLabelText('Nome visual'), {
    target: { value: 'Saved later' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Azul' }));
  fireEvent.click(screen.getByRole('button', { name: 'Salvar' }));
  fireEvent.click(screen.getByRole('button', { name: 'Dashboard' }));
  await act(async () =>
    resolve({
      ...state,
      brand: { name: 'Saved later', logo: '', accent: 'blue' },
    }),
  );
  expect(screen.getByLabelText('Selecionar empresa')).toHaveTextContent(
    'Saved later',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Abrir configurações' }));
  expect(screen.getByLabelText('Nome visual')).toHaveValue('Saved later');
});

it('narrow navigation has an explicit close action that returns focus', async () => {
  render(<App />);
  const trigger = await screen.findByRole('button', { name: 'Navegação' });
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute('aria-expanded', 'true');
  fireEvent.click(screen.getByRole('button', { name: 'Fechar navegação' }));
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  expect(trigger).toHaveFocus();
});
