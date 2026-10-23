import { render, screen, fireEvent, act } from '@testing-library/react';
import { vi, describe, it, expect, afterEach, beforeEach } from 'vitest';
import App from './App';
import { access, type Result } from './presentation';
vi.mock('./presentation', () => ({
  access: {
    status: vi.fn(),
    register: vi.fn(),
    login: vi.fn(),
    recover: vi.fn(),
    logout: vi.fn(),
    createCompany: vi.fn(),
    selectCompany: vi.fn(),
    setTheme: vi.fn(),
    renewRecovery: vi.fn(),
    copyRecovery: vi.fn(),
    ackRecovery: vi.fn(),
    getBrand: vi.fn(),
    saveBrand: vi.fn(),
    saveRecovery: vi.fn(),
  },
}));
const snapshot = (authenticated = false): Result => ({
  code: 'OK',
  request_id: 'test',
  instance_id: 'first',
  activation_id: 'a',
  profile_id: authenticated ? 'company-one' : '',
  heartbeat: 8,
  setup: !authenticated,
  authenticated,
  name: authenticated ? 'technical' : '',
  theme: 'light',
  recovery: '',
  companies: authenticated
    ? [{ id: 'company-one', name: 'Fixture Alpha' }]
    : [],
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.getBrand).mockResolvedValue({
    ...snapshot(true),
    brand: { name: '', logo: '', accent: 'teal' },
  });
  vi.mocked(access.ackRecovery).mockResolvedValue(snapshot());
});
afterEach(() => vi.useRealTimers());
async function enterCredentials() {
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'technical' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'isolated test password' },
  });
}
describe('BF01 real-use presentation', () => {
  it('shows service status and prevents login while disconnected without synthetic controls', async () => {
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      code: 'SERVICE_UNAVAILABLE',
    });
    render(<App />);
    expect(screen.getByText('Aguardando serviço local')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeDisabled();
    expect(screen.queryByText('Consultar')).not.toBeInTheDocument();
  });
  it('registers, clears password, shows recovery once and returns to login', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot());
    vi.mocked(access.register).mockResolvedValue({
      ...snapshot(),
      setup: false,
      recovery: 'a'.repeat(64),
    });
    render(<App />);
    await screen.findByText('Serviço ativo');
    fireEvent.click(screen.getByRole('button', { name: 'Criar conta' }));
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Criar conta local' }));
    await screen.findByText('Guarde seu código');
    expect(access.register).toHaveBeenCalledWith(
      'technical',
      'isolated test password',
    );
    fireEvent.click(screen.getByText('Guardei meu código'));
    await act(async () => {});
    expect(access.ackRecovery).toHaveBeenCalledOnce();
    expect(
      screen.queryByLabelText('Código de recuperação'),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText('Senha', { exact: true })).toHaveValue('');
  });
  it('enters a real company and shows unavailable readings instead of invented traffic', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    render(<App />);
    await screen.findByRole('heading', { name: 'Nenhum equipamento' });
    expect(
      screen.getByRole('button', { name: 'Adicionar equipamento' }),
    ).toBeVisible();
    expect(screen.queryByText('412')).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Selecionar empresa'));
    expect(screen.getByRole('heading', { name: 'Empresas' })).toBeVisible();
  });
  it('keeps login failure readable across successful polls and clears secrets', async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue({ ...snapshot(), setup: false });
    vi.mocked(access.login).mockResolvedValue({
      ...snapshot(),
      code: 'UNAUTHENTICATED',
    });
    render(<App />);
    await act(async () => {});
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    await act(async () => {});
    expect(screen.getByRole('alert')).toHaveTextContent('não conferem');
    expect(screen.getByLabelText('Senha', { exact: true })).toHaveValue('');
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(screen.getByRole('alert')).toHaveTextContent('não conferem');
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      code: 'SERVICE_UNAVAILABLE',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    expect(screen.getByRole('alert')).toHaveTextContent('não conferem');
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: false,
      instance_id: 'reconnected',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(screen.getByRole('alert')).toHaveTextContent('não conferem');
  });
  it('does not claim logout when durable revocation fails', async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    vi.mocked(access.logout).mockResolvedValue({
      ...snapshot(true),
      code: 'SERVICE_UNAVAILABLE',
    });
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
    fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
    await act(async () => {});
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(true),
      code: 'SERVICE_UNAVAILABLE',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(screen.getAllByRole('alert')[0]).toHaveTextContent('serviço local');
    expect(
      screen.getByRole('heading', { name: 'Nenhum equipamento' }),
    ).toBeVisible();
    expect(
      screen.queryByRole('heading', { name: 'Entrar' }),
    ).not.toBeInTheDocument();
  });
  it('stops polling on unmount and ignores a late operation from the old component', async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue(snapshot());
    let complete!: (r: Result) => void;
    vi.mocked(access.login).mockReturnValue(
      new Promise((r) => {
        complete = r;
      }),
    );
    const old = render(<App />);
    await act(async () => {});
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    old.unmount();
    render(<App />);
    await act(async () => {});
    await act(async () => complete(snapshot(true)));
    expect(screen.getByRole('heading', { name: 'Entrar' })).toBeVisible();
    expect(
      screen.queryByRole('heading', { name: 'Nenhum equipamento' }),
    ).not.toBeInTheDocument();
  });
  it('shows loading, prevents double submission and persists theme through daemon binding', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    let complete!: (r: Result) => void;
    vi.mocked(access.setTheme).mockReturnValue(
      new Promise((r) => {
        complete = r;
      }),
    );
    render(<App />);
    await screen.findByRole('heading', { name: 'Nenhum equipamento' });
    fireEvent.click(
      screen.getByRole('button', { name: 'Abrir configurações' }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Escuro' }));
    expect(screen.getByRole('button', { name: 'Escuro' })).toBeDisabled();
    await act(async () => complete({ ...snapshot(true), theme: 'dark' }));
    expect(screen.getByRole('button', { name: 'Escuro' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(access.setTheme).toHaveBeenCalledWith('dark');
  });
  it('discards a rejected login after switching to recovery context', async () => {
    vi.mocked(access.status).mockResolvedValue({ ...snapshot(), setup: false });
    let reject!: (reason: Error) => void;
    vi.mocked(access.login).mockReturnValue(
      new Promise((_, r) => {
        reject = r;
      }),
    );
    render(<App />);
    await screen.findByText('Serviço ativo');
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    fireEvent.click(screen.getByRole('button', { name: 'Recuperar acesso' }));
    await act(async () => reject(new Error('old request')));
    expect(
      screen.getByRole('heading', { name: 'Recuperar acesso' }),
    ).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Nova senha')).toHaveValue('');
  });
  it('keeps branding draft local until confirmed save, cancel restores saved values', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    render(<App />);
    await screen.findByRole('heading', { name: 'Nenhum equipamento' });
    fireEvent.click(
      screen.getByRole('button', { name: 'Abrir configurações' }),
    );
    fireEvent.change(screen.getByLabelText('Nome visual', { exact: true }), {
      target: { value: 'Draft only' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Violeta' }));
    expect(screen.getByRole('button', { name: 'Salvar' })).toBeEnabled();
    expect(access.saveBrand).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Selecionar empresa')).toHaveTextContent(
      'Fixture Alpha',
    );
    expect(screen.queryByText('412')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(screen.getByLabelText('Nome visual', { exact: true })).toHaveValue(
      '',
    );
    expect(screen.getByRole('button', { name: 'Turquesa' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(access.setTheme).not.toHaveBeenCalled();
  });
  it('keeps the one-time recovery code until acknowledgement across a daemon restart', async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue(snapshot());
    vi.mocked(access.register).mockResolvedValue({
      ...snapshot(),
      setup: false,
      recovery: 'b'.repeat(64),
    });
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole('button', { name: 'Criar conta' }));
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Criar conta local' }));
    await act(async () => {});
    expect(screen.getByLabelText('Código de recuperação')).toBeVisible();
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      code: 'SERVICE_UNAVAILABLE',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: false,
      instance_id: 'new-daemon',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(screen.getByLabelText('Código de recuperação')).toHaveValue(
      Array(8).fill('bbbbbbbb').join('-'),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    await act(async () => {});
    expect(
      screen.queryByLabelText('Código de recuperação'),
    ).not.toBeInTheDocument();
  });
});
