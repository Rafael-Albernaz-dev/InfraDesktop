import { render, screen, fireEvent, act, within } from '@testing-library/react';
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
  if (screen.queryByLabelText('Confirmar senha'))
    fireEvent.change(screen.getByLabelText('Confirmar senha'), {
      target: { value: 'isolated test password' },
    });
}
const emptyHeading = 'Visão geral';
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
    await screen.findByText('Guarde seu novo código');
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
    fireEvent.click(screen.getByRole('link', { name: 'Equipamentos' }));
    expect(
      await screen.findByRole('button', { name: 'Adicionar equipamento' }),
    ).toBeVisible();
    expect(screen.queryByText('412')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('link', { name: /Fixture Alpha/ }));
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
    fireEvent.click(screen.getByRole('menuitem', { name: 'Sair da conta' }));
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
        name: 'Visão geral',
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
    const { container } = render(<App />);
    await screen.findByRole('heading', { name: emptyHeading });
    fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Tema escuro' }));
    fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
    expect(
      screen.getByRole('menuitem', { name: 'Tema escuro' }),
    ).toBeDisabled();
    await act(async () =>
      deferred.resolve({ ...snapshot(true), theme: 'dark' }),
    );
    expect(container.querySelector('.app')).toHaveAttribute(
      'data-theme',
      'dark',
    );
    expect(access.setTheme).toHaveBeenCalledOnce();
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
it('supports company, overview, equipment, credential and settings routes with coherent paths', async () => {
  vi.mocked(access.status).mockResolvedValue(snapshot(true));
  render(<App />);
  await screen.findByRole('heading', { name: 'Visão geral' });
  const path = () => screen.getByRole('navigation', { name: 'Caminho' });
  fireEvent.click(screen.getByRole('link', { name: 'Equipamentos' }));
  expect(path()).toHaveTextContent('Equipamentos');
  fireEvent.click(screen.getByRole('link', { name: 'Credenciais' }));
  expect(path()).toHaveTextContent('Equipamentos›Credenciais');
  fireEvent.click(within(path()).getByRole('link', { name: 'Equipamentos' }));
  expect(path()).toHaveTextContent('Equipamentos');
  fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
  expect(path()).toHaveTextContent('Configurações');
  expect(
    screen.queryByRole('button', { name: 'Aparência' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('link', { name: 'Topologia' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('link', { name: 'Sessões PPPoE' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('link', { name: 'Visão geral' }));
  expect(path()).toHaveTextContent('Visão geral');
  fireEvent.click(screen.getByRole('link', { name: /Fixture Alpha/ }));
  expect(
    await screen.findByRole('heading', { name: 'Empresas' }),
  ).toBeVisible();
});
