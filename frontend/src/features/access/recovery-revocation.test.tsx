import {
  render,
  screen,
  fireEvent,
  act,
  cleanup,
  within,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import App from '../../app/App';
import { access, type Result } from '../../platform/wails/presentation';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
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
it('removes the transient code when the account session is actually revoked', async () => {
  vi.useFakeTimers();
  try {
    vi.mocked(access.renewRecovery).mockResolvedValue({
      ...state,
      recovery: 'c'.repeat(64),
    });
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
    fireEvent.click(screen.getByRole('button', { name: 'Gerar novo código' }));
    submit();
    await act(async () => {});
    expect(
      screen.getByLabelText('Novo código de recuperação'),
    ).toBeInTheDocument();
    vi.mocked(access.status).mockResolvedValue({
      ...state,
      authenticated: false,
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    expect(
      screen.queryByLabelText('Novo código de recuperação'),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeInTheDocument();
  } finally {
    vi.useRealTimers();
  }
});
