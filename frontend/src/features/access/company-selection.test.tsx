import {
  render,
  screen,
  fireEvent,
  act,
  cleanup,
} from '@testing-library/react';
import { vi, it, expect, beforeEach, afterEach } from 'vitest';
import App from '../../app/App';

import {
  access,
  equipmentAPI,
  type Result,
} from '../../platform/wails/presentation';
vi.mock('../../platform/wails/presentation', () => ({
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
    fireEvent.click(screen.getByRole('cell', { name: 'Audit Company' }));
    await act(async () => {});
    expect(screen.getByRole('link', { name: 'Visão geral' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(
      screen.getByRole('link', { name: /Audit Company/ }),
    ).toHaveTextContent('Audit Company');
    expect(container.querySelector('.app')).toHaveAttribute(
      'data-route',
      'overview',
    );
    expect(screen.getByRole('heading', { name: 'Visão geral' })).toBeVisible();
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
    expect(screen.getByRole('link', { name: 'Visão geral' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(
      screen.getByRole('link', { name: /Audit Company/ }),
    ).toHaveTextContent('Audit Company');
    expect(access.selectCompany).toHaveBeenCalledExactlyOnceWith('one');
  } finally {
    vi.useRealTimers();
  }
});
