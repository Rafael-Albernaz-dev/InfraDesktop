import {
  render,
  screen,
  fireEvent,
  act,
  cleanup,
  within,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import App from './App';
import { access, type Result } from './presentation';
vi.mock('./presentation', () => ({
  access: {
    status: vi.fn(),
    renewRecovery: vi.fn(),
    copyRecovery: vi.fn(),
    ackRecovery: vi.fn(),
    getBrand: vi.fn(),
    saveBrand: vi.fn(),
    saveRecovery: vi.fn(),
    setTheme: vi.fn(),
    selectCompany: vi.fn(),
    logout: vi.fn(),
  },
}));
const state: Result = {
  code: 'OK',
  request_id: 'security',
  instance_id: 'instance',
  activation_id: 'a',
  profile_id: '',
  heartbeat: 0,
  setup: false,
  authenticated: true,
  name: 'technical',
  theme: 'light',
  recovery: '',
  companies: [],
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.status).mockResolvedValue(state);
  vi.mocked(access.setTheme).mockResolvedValue({ ...state, theme: 'dark' });
  vi.mocked(access.ackRecovery).mockResolvedValue(state);
});
afterEach(cleanup);
async function openSecurity() {
  render(<App />);
  fireEvent.click(await screen.findByRole('button', { name: 'Configurações' }));
  fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
}
function submit() {
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'isolated test password' },
    },
  );
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', {
      name: 'Gerar novo código',
    }),
  );
}
it('works without company, retains pending issuance and one-time code until ack, and copies narrowly', async () => {
  let complete!: (r: Result) => void;
  vi.mocked(access.renewRecovery).mockReturnValue(
    new Promise((resolve) => {
      complete = resolve;
    }),
  );
  vi.mocked(access.copyRecovery).mockResolvedValue('OK');
  await openSecurity();
  submit();
  fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
  fireEvent.click(screen.getByRole('link', { name: /Selecionar empresa/ }));
  expect(
    screen.getByRole('dialog', { name: 'Gerar novo código' }),
  ).toBeInTheDocument();
  await act(async () => complete({ ...state, recovery: 'a'.repeat(64) }));
  expect(
    within(screen.getByRole('dialog')).queryByLabelText('Senha atual'),
  ).not.toBeInTheDocument();
  const field = screen.getByLabelText('Novo código de recuperação');
  expect(field).toHaveValue(Array(8).fill('aaaaaaaa').join('-'));
  expect(screen.getByLabelText('Novo código de recuperação')).toHaveValue(
    Array(8).fill('aaaaaaaa').join('-'),
  );
  fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
  expect(screen.getByRole('dialog')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Copiar código' }));
  await screen.findByText('Código copiado.');
  expect(access.copyRecovery).toHaveBeenCalledWith();
  fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
  await act(async () => {});
  expect(access.ackRecovery).toHaveBeenCalledOnce();
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
it('does not claim copy success on clipboard failure, permits manual selection and preserves code on failed ack', async () => {
  vi.mocked(access.renewRecovery).mockResolvedValue({
    ...state,
    recovery: 'b'.repeat(64),
  });
  vi.mocked(access.copyRecovery).mockResolvedValue('CLIPBOARD_UNAVAILABLE');
  vi.mocked(access.ackRecovery).mockResolvedValue({
    ...state,
    code: 'SERVICE_UNAVAILABLE',
  });
  await openSecurity();
  submit();
  await screen.findByLabelText('Novo código de recuperação');
  fireEvent.click(screen.getByRole('button', { name: 'Copiar código' }));
  await screen.findByText(/Selecione e copie/);
  expect(screen.getByLabelText('Novo código de recuperação')).toHaveAttribute(
    'readonly',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
  await screen.findByRole('alert');
  expect(
    screen.getByLabelText('Novo código de recuperação'),
  ).toBeInTheDocument();
});
it('clears password, gives uncertainty explanation, and only explicitly reissues after failure', async () => {
  vi.mocked(access.renewRecovery).mockResolvedValue({
    ...state,
    code: 'DEADLINE_EXCEEDED',
  });
  await openSecurity();
  submit();
  await screen.findByText(/Não recebemos a confirmação/);
  expect(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
  ).toHaveValue('');
  expect(access.renewRecovery).toHaveBeenCalledTimes(1);
  fireEvent.change(
    within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    {
      target: { value: 'isolated test password' },
    },
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Gerar outro código novo' }),
  );
  await act(async () => {});
  expect(access.renewRecovery).toHaveBeenLastCalledWith(
    'isolated test password',
    true,
  );
});
for (const failure of ['DEADLINE_EXCEEDED', 'reject'] as const) {
  it(`retains issuance uncertainty across panel unmount/reopen after ${failure}`, async () => {
    if (failure === 'reject')
      vi.mocked(access.renewRecovery).mockRejectedValueOnce(
        new Error('lost reply'),
      );
    else
      vi.mocked(access.renewRecovery).mockResolvedValueOnce({
        ...state,
        code: failure,
      });
    vi.mocked(access.renewRecovery).mockResolvedValueOnce({
      ...state,
      recovery: 'e'.repeat(64),
    });
    await openSecurity();
    submit();
    await screen.findByText(/Não recebemos a confirmação/);
    expect(access.renewRecovery).toHaveBeenNthCalledWith(
      1,
      'isolated test password',
      false,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
    expect(screen.getByText(/Não recebemos a confirmação/)).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
    ).toHaveValue('');
    expect(access.renewRecovery).toHaveBeenCalledTimes(1);
    fireEvent.change(
      within(screen.getByRole('dialog')).getByLabelText('Senha atual'),
      {
        target: { value: 'isolated test password' },
      },
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Gerar outro código novo' }),
    );
    await screen.findByLabelText('Novo código de recuperação');
    expect(access.renewRecovery).toHaveBeenNthCalledWith(
      2,
      'isolated test password',
      true,
    );
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    expect(
      screen.getByLabelText('Novo código de recuperação'),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Guardei meu código' }));
    await act(async () => {});
    fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
    expect(
      screen.queryByText(/Não recebemos a confirmação/),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('dialog')).getByRole('button', {
        name: 'Gerar novo código',
      }),
    ).toBeInTheDocument();
  });
}
