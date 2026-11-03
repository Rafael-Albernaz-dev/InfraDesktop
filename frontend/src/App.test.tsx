import { render, screen, fireEvent, act } from '@testing-library/react';
import { vi, describe, it, expect, afterEach, beforeEach } from 'vitest';
import App from './App';
import { access, equipmentAPI, type Result } from './presentation';

const accessMock = vi.hoisted(() => {
  const s =
    'status,register,login,recover,logout,createCompany,selectCompany,' +
    'setTheme,renewRecovery,copyRecovery,ackRecovery,getBrand,saveBrand,saveRecovery';
  return Object.fromEntries(s.split(',').map((m) => [m, vi.fn()]));
});
vi.mock('./presentation', () => ({
  equipmentAPI: { list: vi.fn() },
  access: accessMock,
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
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    code: 'OK',
    profile_id: 'company-one',
    activation_id: 'a',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      total_devices: 0,
      vault_available: true,
    },
  });
  vi.mocked(access.getBrand).mockResolvedValue({
    ...snapshot(true),
    brand: { name: '', logo: '', accent: 'teal' },
  });
  vi.mocked(access.ackRecovery).mockResolvedValue(snapshot());
});
afterEach(() => vi.useRealTimers());
const advance = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const statusMock = (code = 'OK', extra: Partial<Result> = {}) =>
  vi.mocked(access.status).mockResolvedValue({ ...snapshot(), code, ...extra });
function defer<T>() {
  let resolve!: (v: T) => void, reject!: (e: unknown) => void;
  const p = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { p, resolve, reject };
}

async function enterCredentials() {
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'technical' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'isolated test password' },
  });
}

const emptyHeading = 'Nenhum equipamento cadastrado';

describe('BF01 real-use presentation', () => {
  it('shows service status and prevents login while disconnected without synthetic controls', async () => {
    statusMock('SERVICE_UNAVAILABLE');
    render(<App />);
    expect(screen.getByText('Aguardando serviço local')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeDisabled();
    expect(screen.queryByText('Consultar')).not.toBeInTheDocument();
  });
  it('registers, clears password, shows recovery once and returns to login', async () => {
    statusMock();
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
    await screen.findByRole('heading', { name: emptyHeading });
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
    const expectAlert = () =>
      expect(screen.getByRole('alert')).toHaveTextContent('não conferem');
    expectAlert();
    expect(screen.getByLabelText('Senha', { exact: true })).toHaveValue('');
    await advance(3000);
    expectAlert();
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      code: 'SERVICE_UNAVAILABLE',
    });
    await advance(1000);
    expectAlert();
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: false,
      instance_id: 'reconnected',
    });
    await advance(3000);
    expectAlert();
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
    await advance(3000);
    expect(screen.getAllByRole('alert')[0]).toHaveTextContent('serviço local');
    expect(
      screen.getByRole('heading', {
        name: 'Nenhum equipamento cadastrado',
      }),
    ).toBeVisible();
    expect(
      screen.queryByRole('heading', { name: 'Entrar' }),
    ).not.toBeInTheDocument();
  });
  it('stops polling on unmount and ignores a late operation from the old component', async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue(snapshot());
    const deferred = defer<Result>();
    vi.mocked(access.login).mockReturnValue(deferred.p);
    const old = render(<App />);
    await act(async () => {});
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    old.unmount();
    render(<App />);
    await act(async () => {});
    await act(async () => deferred.resolve(snapshot(true)));
    expect(screen.getByRole('heading', { name: 'Entrar' })).toBeVisible();
    expect(
      screen.queryByRole('heading', { name: 'Nenhum equipamento cadastrado' }),
    ).not.toBeInTheDocument();
  });
  it('shows loading, prevents double submission and persists theme through daemon binding', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    const deferred = defer<Result>();
    vi.mocked(access.setTheme).mockReturnValue(deferred.p);
    render(<App />);
    await screen.findByRole('heading', { name: emptyHeading });
    fireEvent.click(
      screen.getByRole('button', { name: 'Abrir configurações' }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Escuro' }));
    expect(screen.getByRole('button', { name: 'Escuro' })).toBeDisabled();
    await act(async () =>
      deferred.resolve({ ...snapshot(true), theme: 'dark' }),
    );
    expect(screen.getByRole('button', { name: 'Escuro' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(access.setTheme).toHaveBeenCalledWith('dark');
  });
  it('discards a rejected login after switching to recovery context', async () => {
    vi.mocked(access.status).mockResolvedValue({ ...snapshot(), setup: false });
    const deferred = defer<Result>();
    vi.mocked(access.login).mockReturnValue(deferred.p);
    render(<App />);
    await screen.findByText('Serviço ativo');
    await enterCredentials();
    fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    fireEvent.click(screen.getByRole('button', { name: 'Recuperar acesso' }));
    await act(async () => deferred.reject(new Error('old request')));
    expect(
      screen.getByRole('heading', { name: 'Recuperar acesso' }),
    ).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Nova senha')).toHaveValue('');
  });
  it('keeps branding draft local until confirmed save, cancel restores saved values', async () => {
    vi.mocked(access.status).mockResolvedValue(snapshot(true));
    render(<App />);
    await screen.findByRole('heading', { name: emptyHeading });
    fireEvent.click(
      screen.getByRole('button', { name: 'Abrir configurações' }),
    );
    const visual = screen.getByLabelText('Nome visual', { exact: true });
    fireEvent.change(visual, { target: { value: 'Draft only' } });
    fireEvent.click(screen.getByRole('button', { name: 'Violeta' }));
    expect(screen.getByRole('button', { name: 'Salvar' })).toBeEnabled();
    expect(access.saveBrand).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Selecionar empresa')).toHaveTextContent(
      'Fixture Alpha',
    );
    expect(screen.queryByText('412')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(visual).toHaveValue('');
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
    const recoveryCode = () => screen.queryByLabelText('Código de recuperação');
    expect(recoveryCode()).toBeVisible();
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      code: 'SERVICE_UNAVAILABLE',
    });
    await advance(1000);
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: false,
      instance_id: 'new-daemon',
    });
    await advance(3000);
    expect(recoveryCode()).toHaveValue(Array(8).fill('bbbbbbbb').join('-'));
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    await act(async () => {});
    expect(recoveryCode()).not.toBeInTheDocument();
  });
});

it('C01b confirmed unauthentication clears expired receipts; transport failure retains them before same-context login', async () => {
  const { pendingStarts } = await import('./equipment-receipts');
  vi.useFakeTimers();
  vi.mocked(access.status).mockResolvedValue(snapshot(true));
  render(<App />);
  await act(async () => {});
  for (let i = 0; i < 128; i++)
    pendingStarts.set('expired-' + i, 'operation-' + i);
  const setStatus = (code: string, setup = true) =>
    vi.mocked(access.status).mockResolvedValue({ ...snapshot(), code, setup });
  setStatus('SERVICE_UNAVAILABLE');
  await advance(1000);
  expect(pendingStarts.size).toBe(128);
  setStatus('UNAUTHENTICATED', false);
  await advance(2000);
  expect(pendingStarts.size).toBe(0);
  setStatus('OK', false);
  await advance(4000);
  vi.mocked(access.login).mockResolvedValue(snapshot(true));
  await enterCredentials();
  fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
  await act(async () => {});
  expect(access.login).toHaveBeenCalledOnce();
  expect(pendingStarts.size).toBe(0);
});

it('supports named route navigation across companies, overview, equipment, pppoe, topology, settings and back navigation', async () => {
  vi.mocked(access.status).mockResolvedValue(snapshot(true));
  render(<App />);
  await screen.findByRole('button', { name: 'Selecionar empresa' });
  const nav = () => screen.getByRole('navigation', { name: 'Caminho da tela' });
  const btn = (name: string) => screen.getByRole('button', { name });
  const step = (target: string, exp: string, bk = false) => {
    fireEvent.click(btn(target));
    expect(nav()).toHaveTextContent(exp);
    if (bk) expect(btn('Voltar à tela anterior')).toBeInTheDocument();
  };

  expect(nav()).toHaveTextContent('Visão geral');
  step('Equipamentos', 'Equipamentos', true);
  step('Voltar à tela anterior', 'Visão geral');
  step('Sessões PPPoE', 'Sessões PPPoE', true);
  step('Voltar à tela anterior', 'Equipamentos', true);
  step('Voltar à tela anterior', 'Visão geral');
  step('Topologia', 'Topologia', true);
  step('Voltar à tela anterior', 'Visão geral');
  fireEvent.click(btn('Configurações'));
  fireEvent.click(btn('Aparência'));
  expect(nav()).toHaveTextContent('Configurações');
  expect(btn('Voltar à tela anterior')).toBeInTheDocument();
  step('Voltar à tela anterior', 'Visão geral');
  step('Selecionar empresa', 'Empresas');
  expect(
    screen.queryByRole('button', { name: 'Voltar à tela anterior' }),
  ).not.toBeInTheDocument();
});
