import {
  render,
  screen,
  fireEvent,
  act,
  cleanup,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import App from './App';
import { Appearance } from './desktop';
import { access, equipmentAPI, type Result } from './presentation';
vi.mock('./presentation', () => ({
  equipmentAPI: { list: vi.fn() },
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
  request_id: 'audit',
  instance_id: 'audit',
  activation_id: 'a',
  profile_id: authenticated ? 'one' : '',
  heartbeat: 1,
  setup: !authenticated,
  authenticated,
  name: authenticated ? 'audit' : '',
  theme: 'light',
  recovery: '',
  companies: authenticated ? [{ id: 'one', name: 'Audit Company' }] : [],
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(equipmentAPI.list).mockImplementation(
    async (profile, activation) => ({
      code: 'OK',
      profile_id: profile,
      activation_id: activation,
      data: {
        devices: [],
        credentials: [],
        next_cursor: 0,
        total_devices: 0,
        vault_available: true,
      },
    }),
  );
  vi.mocked(access.getBrand).mockResolvedValue({
    ...snapshot(true),
    brand: { name: '', logo: '', accent: 'teal' },
  });
  vi.mocked(access.ackRecovery).mockResolvedValue(snapshot());
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
for (const mode of ['register', 'recover'] as const) {
  it(`preserves one-time recovery after pending ${mode} and attempted navigation`, async () => {
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: mode === 'register',
    });
    let complete!: (r: Result) => void;
    vi.mocked(access[mode]).mockReturnValue(
      new Promise((resolve) => {
        complete = resolve;
      }),
    );
    render(<App />);
    await screen.findByText('Serviço ativo');
    fireEvent.click(
      screen.getByRole('button', {
        name: mode === 'register' ? 'Criar conta' : 'Recuperar acesso',
      }),
    );
    fireEvent.change(screen.getByLabelText('Usuário'), {
      target: { value: 'audit' },
    });
    fireEvent.change(
      screen.getByLabelText(mode === 'register' ? 'Senha' : 'Nova senha', {
        exact: true,
      }),
      { target: { value: 'audit isolated password' } },
    );
    if (mode === 'recover')
      fireEvent.change(screen.getByLabelText('Código de recuperação'), {
        target: { value: 'a'.repeat(64) },
      });
    fireEvent.click(
      screen.getByRole('button', {
        name: mode === 'register' ? 'Criar conta local' : 'Redefinir senha',
      }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Voltar ao login' }));
    await act(async () =>
      complete({ ...snapshot(), setup: false, recovery: 'b'.repeat(64) }),
    );
    expect(
      screen.queryByLabelText('Código de recuperação'),
      'committed operation must still disclose its one-time code',
    ).toBeInTheDocument();
  });
}
it('does not assign failed logout feedback to a new company dialog', async () => {
  vi.mocked(access.status).mockResolvedValue({
    ...snapshot(true),
    profile_id: '',
  });
  vi.mocked(access.logout).mockResolvedValue({
    ...snapshot(true),
    profile_id: '',
    code: 'DEADLINE_EXCEEDED',
  });
  render(<App />);
  await screen.findByRole('heading', { name: 'Empresas' });
  fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
  fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
  fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
  await act(async () => {});
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  fireEvent.click(screen.getByRole('button', { name: 'Nova empresa' }));
  expect(
    screen.getByRole('dialog').querySelector('[role="alert"]'),
    'new company must not receive logout error',
  ).toBeNull();
});
it('does not resurrect canceled branding from an old FileReader', async () => {
  ControlledReader.readers = [];
  vi.stubGlobal('FileReader', Reader);
  const { container } = render(
    <Appearance
      brand={{ name: 'Original', logo: '', accent: 'teal' }}
      theme="light"
      setTheme={() => {}}
      busy={false}
    />,
  );
  fireEvent.change(screen.getByLabelText('Nome visual', { exact: true }), {
    target: { value: 'Canceled draft' },
  });
  fireEvent.change(container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(['audit'], 'audit.png', { type: 'image/png' })],
    },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(screen.getByLabelText('Nome visual', { exact: true })).toHaveValue(
    'Original',
  );
  await act(async () =>
    ControlledReader.readers[0].finish('data:image/png;base64,audit'),
  );
  expect(
    screen.getByLabelText('Nome visual', { exact: true }),
    'old reader must not restore canceled draft',
  ).toHaveValue('Original');
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
async function accessForm(mode: 'register' | 'recover') {
  fireEvent.click(
    screen.getByRole('button', {
      name: mode === 'register' ? 'Criar conta' : 'Recuperar acesso',
    }),
  );
  fillAccess(mode);
}
function fillAccess(mode: 'register' | 'recover') {
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'audit' },
  });
  fireEvent.change(
    screen.getByLabelText(mode === 'register' ? 'Senha' : 'Nova senha', {
      exact: true,
    }),
    { target: { value: 'audit isolated password' } },
  );
  if (mode === 'recover')
    fireEvent.change(screen.getByLabelText('Código de recuperação'), {
      target: { value: 'a'.repeat(64) },
    });
}
for (const mode of ['register', 'recover'] as const) {
  it(`${mode}: failure unlocks retry, two submits call once, theme and polling retain code until ack`, async () => {
    vi.useFakeTimers();
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: mode === 'register',
    });
    const first = deferred<Result>();
    const second = deferred<Result>();
    vi.mocked(access[mode])
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const { container } = render(<App />);
    await act(async () => {});
    await accessForm(mode);
    const form = container.querySelector('form')!;
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(access[mode]).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole('button', { name: 'Voltar ao login' }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Escuro' }));
    await act(async () => first.reject(new Error('isolated failure')));
    expect(screen.getByRole('alert')).toHaveTextContent('serviço local');
    expect(
      screen.getByRole('button', { name: 'Voltar ao login' }),
    ).toBeEnabled();
    fillAccess(mode);
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(access[mode]).toHaveBeenCalledTimes(2);
    await act(async () =>
      second.resolve({ ...snapshot(), setup: false, recovery: 'c'.repeat(64) }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Claro' }));
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: false,
      instance_id: 'restart',
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(screen.getByLabelText('Código de recuperação')).toHaveTextContent(
      'cccccccc',
    );
    expect(access[mode]).toHaveBeenCalledTimes(2);
    expect(access.setTheme).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    await act(async () => {});
    expect(
      screen.queryByLabelText('Código de recuperação'),
    ).not.toBeInTheDocument();
    vi.useRealTimers();
  });
}
for (const mode of ['register', 'recover'] as const) {
  it(`${mode}: local recovery acknowledgement waits for the adapter and retains code after failure`, async () => {
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(),
      setup: mode === 'register',
    });
    vi.mocked(access[mode]).mockResolvedValue({
      ...snapshot(),
      setup: false,
      recovery: 'd'.repeat(64),
    });
    const ack = deferred<Result>();
    vi.mocked(access.ackRecovery).mockReturnValueOnce(ack.promise);
    const { container } = render(<App />);
    await screen.findByText('Serviço ativo');
    await accessForm(mode);
    fireEvent.submit(container.querySelector('form')!);
    await screen.findByLabelText('Código de recuperação');
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    expect(
      screen.getByRole('button', { name: 'Guardei meu código' }),
    ).toBeDisabled();
    expect(screen.getByLabelText('Código de recuperação')).toHaveTextContent(
      'dddddddd',
    );
    await act(async () =>
      ack.resolve({ ...snapshot(), code: 'SERVICE_UNAVAILABLE' }),
    );
    expect(screen.getByRole('alert')).toHaveTextContent('serviço local');
    expect(screen.getByLabelText('Código de recuperação')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    await act(async () => {});
    expect(access.ackRecovery).toHaveBeenCalledTimes(2);
    expect(
      screen.queryByLabelText('Código de recuperação'),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText('Senha', { exact: true })).toHaveValue('');
  });
}
for (const outcome of ['success', 'failure', 'reject'] as const) {
  it(`company ${outcome} from a closed dialog cannot close or report in the replacement; double submit calls once`, async () => {
    vi.mocked(access.status).mockResolvedValue({
      ...snapshot(true),
      profile_id: '',
    });
    const old = deferred<Result>();
    vi.mocked(access.createCompany)
      .mockReturnValueOnce(old.promise)
      .mockResolvedValueOnce({ ...snapshot(true), profile_id: '' });
    const { container } = render(<App />);
    await screen.findByRole('heading', { name: 'Empresas' });
    fireEvent.click(screen.getByRole('button', { name: 'Nova empresa' }));
    fireEvent.change(screen.getByLabelText('Nome da empresa'), {
      target: { value: 'First' },
    });
    fireEvent.submit(container.querySelector('form')!);
    fireEvent.submit(container.querySelector('form')!);
    expect(access.createCompany).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    // While the first mutation is pending the toolbar stays disabled; opening
    // a different surface still must invalidate the old callback.
    fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
    if (outcome === 'reject')
      await act(async () => old.reject(new Error('old')));
    else
      await act(async () =>
        old.resolve({
          ...snapshot(true),
          profile_id: '',
          code: outcome === 'success' ? 'OK' : 'DEADLINE_EXCEEDED',
        }),
      );
    expect(screen.getByRole('dialog')).toHaveTextContent(
      'Sair desta instalação?',
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    fireEvent.click(screen.getByRole('button', { name: 'Nova empresa' }));
    expect(screen.getByLabelText('Nome da empresa')).toHaveValue('');
    fireEvent.change(screen.getByLabelText('Nome da empresa'), {
      target: { value: 'Second' },
    });
    fireEvent.submit(container.querySelector('form')!);
    fireEvent.submit(container.querySelector('form')!);
    await act(async () => {});
    expect(access.createCompany).toHaveBeenCalledTimes(2);
    expect(access.createCompany).toHaveBeenNthCalledWith(2, 'Second');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
}
it('preauth theme survives service failure/reconnect/modes and keyboard, account theme wins on login, logout keeps presentation', async () => {
  vi.useFakeTimers();
  vi.mocked(access.status).mockResolvedValue(snapshot());
  vi.mocked(access.login).mockResolvedValue({
    ...snapshot(true),
    theme: 'light',
  });
  vi.mocked(access.logout).mockResolvedValue({ ...snapshot(), setup: false });
  const { container } = render(<App />);
  await act(async () => {});
  const dark = screen.getByRole('button', { name: 'Escuro' });
  dark.focus();
  expect(dark).toHaveFocus();
  fireEvent.click(dark);
  expect(dark).toHaveAttribute('aria-pressed', 'true');
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'audit' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'audit isolated password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Claro' }));
  expect(screen.getByLabelText('Senha', { exact: true })).toHaveValue(
    'audit isolated password',
  );
  fireEvent.click(dark);
  vi.mocked(access.status).mockResolvedValue({
    ...snapshot(),
    code: 'SERVICE_UNAVAILABLE',
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1000);
  });
  expect(dark).toBeEnabled();
  vi.mocked(access.status).mockResolvedValue({
    ...snapshot(),
    instance_id: 'reconnected',
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(container.querySelector('.app')).toHaveAttribute('data-theme', 'dark');
  fireEvent.click(screen.getByRole('button', { name: 'Recuperar acesso' }));
  expect(dark).toHaveAttribute('aria-pressed', 'true');
  fireEvent.click(screen.getByRole('button', { name: 'Voltar ao login' }));
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'audit isolated password' },
  });
  fireEvent.submit(container.querySelector('form')!);
  await act(async () => {});
  expect(container.querySelector('.app')).toHaveAttribute(
    'data-theme',
    'light',
  );
  fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
  fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
  fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
  await act(async () => {});
  expect(container.querySelector('.app')).toHaveAttribute(
    'data-theme',
    'light',
  );
  expect(screen.getByRole('button', { name: 'Escuro' })).toBeEnabled();
  expect(access.setTheme).not.toHaveBeenCalled();
  vi.useRealTimers();
});

type PendingReader = {
  result: string;
  onload: null | (() => void);
  aborted: boolean;
  abort: () => void;
  readAsDataURL: () => void;
  finish: (value: string) => void;
};
const ControlledReader = { readers: [] as PendingReader[] };
function Reader() {
  const reader: PendingReader = {
    result: '',
    onload: null,
    aborted: false,
    abort() {
      reader.aborted = true;
    },
    readAsDataURL() {
      ControlledReader.readers.push(reader);
    },
    finish(value: string) {
      reader.result = value;
      reader.onload?.();
    },
  };
  return reader;
}
for (const action of [
  'Cancelar',
  'Restaurar padrão',
  'Remover',
  'unmount',
] as const) {
  it(`invalidates pending logo on ${action}`, async () => {
    ControlledReader.readers = [];
    vi.stubGlobal('FileReader', Reader);
    const view = render(
      <Appearance
        brand={{ name: 'Original', logo: 'data:old', accent: 'teal' }}
        theme="light"
        setTheme={() => {}}
        busy={false}
      />,
    );
    fireEvent.change(screen.getByLabelText('Nome visual', { exact: true }), {
      target: { value: 'Draft' },
    });
    fireEvent.change(view.container.querySelector('input[type=file]')!, {
      target: { files: [new File(['x'], 'x.png', { type: 'image/png' })] },
    });
    const reader = ControlledReader.readers[0];
    if (action === 'unmount') view.unmount();
    else fireEvent.click(screen.getByRole('button', { name: action }));
    expect(reader.aborted).toBe(true);
    await act(async () => reader.finish('data:late'));
    expect(view.container.querySelector('img[src="data:late"]')).toBeNull();
    if (action === 'Cancelar')
      expect(screen.getByLabelText('Nome visual', { exact: true })).toHaveValue(
        'Original',
      );
  });
}
it('latest logo wins out of order without undoing concurrent name/accent changes', async () => {
  ControlledReader.readers = [];
  vi.stubGlobal('FileReader', Reader);
  const { container } = render(
    <Appearance
      brand={{ name: 'Original', logo: '', accent: 'teal' }}
      theme="light"
      setTheme={() => {}}
      busy={false}
    />,
  );
  const input = container.querySelector('input[type=file]')!;
  const file = new File(['x'], 'x.png', { type: 'image/png' });
  fireEvent.change(input, { target: { files: [file] } });
  fireEvent.change(input, { target: { files: [file] } });
  fireEvent.change(screen.getByLabelText('Nome visual', { exact: true }), {
    target: { value: 'Recent name' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Violeta' }));
  await act(async () => ControlledReader.readers[1].finish('data:new'));
  await act(async () => ControlledReader.readers[0].finish('data:old'));
  expect(screen.getByLabelText('Nome visual', { exact: true })).toHaveValue(
    'Recent name',
  );
  expect(screen.getByRole('button', { name: 'Violeta' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  expect(container.querySelector('img')).toHaveAttribute('src', 'data:new');
});
it('can cancel a logo read before any other draft edit', async () => {
  ControlledReader.readers = [];
  vi.stubGlobal('FileReader', Reader);
  const { container } = render(
    <Appearance
      brand={{ name: 'Original', logo: '', accent: 'teal' }}
      theme="light"
      setTheme={() => {}}
      busy={false}
    />,
  );
  fireEvent.change(container.querySelector('input[type=file]')!, {
    target: { files: [new File(['x'], 'x.png', { type: 'image/png' })] },
  });
  expect(screen.getByRole('button', { name: 'Cancelar' })).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  await act(async () => ControlledReader.readers[0].finish('data:late'));
  expect(container.querySelector('img')).toBeNull();
});
it('a valid company selection still opens its workspace after the service changes profile context', async () => {
  vi.useFakeTimers();
  let current = {
    ...snapshot(true),
    profile_id: '',
  };
  vi.mocked(access.status).mockImplementation(async () => current);
  vi.mocked(access.selectCompany).mockImplementation(async (profile) => {
    expect(profile).toBe('one');
    current = { ...snapshot(true), activation_id: 'selected-company' };
    return current;
  });
  try {
    const { container } = render(<App />);
    await act(async () => {});
    expect(screen.getByRole('heading', { name: 'Empresas' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Abrir empresa' }));
    await act(async () => {});
    expect(screen.getByRole('button', { name: 'Dashboard' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(
      screen.getByRole('button', { name: 'Selecionar empresa' }),
    ).toHaveTextContent('Audit Company');
    expect(container.querySelector('.app')).toHaveAttribute('data-page', '3');
    expect(
      screen.getByRole('heading', { name: 'Operação da rede' }),
    ).toBeVisible();
    expect(equipmentAPI.list).toHaveBeenCalledWith(
      'one',
      'selected-company',
      '',
      0,
      false,
    );
    const polls = vi.mocked(access.status).mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(vi.mocked(access.status).mock.calls.length).toBeGreaterThan(polls);
    expect(current.profile_id).toBe('one');
    expect(screen.getByRole('button', { name: 'Dashboard' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(
      screen.getByRole('button', { name: 'Selecionar empresa' }),
    ).toHaveTextContent('Audit Company');
    expect(access.selectCompany).toHaveBeenCalledExactlyOnceWith('one');
  } finally {
    vi.useRealTimers();
  }
});
