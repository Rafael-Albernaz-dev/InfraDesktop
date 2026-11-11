import {
  act,
  cleanup,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { createPerformAction } from '../../app/app-perform';
import {
  createBrandSaver,
  useAccessSession,
  useBrandState,
} from './access-session';
import { access, type Result } from '../../platform/wails/presentation';
import { BrandDraftHarness as Appearance } from '../../app/test-brand-draft';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
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

const noop = () => {};
function useRetainedBrand() {
  const session = useAccessSession(noop, noop);
  const brand = useBrandState(session.view, session.connected);
  return {
    session,
    brand,
    save: createBrandSaver(session.view, brand, createPerformAction(session)),
  };
}
it('retains brand data and the submitted draft on failure, then saves without renaming the company catalog', async () => {
  vi.mocked(access.saveBrand)
    .mockResolvedValueOnce({ ...state, code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce({
      ...state,
      brand: { name: 'Visual X', logo: '', accent: 'rose' },
    });
  const hook = renderHook(useRetainedBrand);
  await waitFor(() => expect(hook.result.current.save).toBeTypeOf('function'));
  const draft = { name: 'Visual X', logo: '', accent: 'rose' };
  let success: boolean | undefined;
  await act(async () => {
    success = await hook.result.current.save!(draft);
  });
  expect(success).toBe(false);
  expect(hook.result.current.session.error).toContain('serviço');
  expect(draft).toEqual({ name: 'Visual X', logo: '', accent: 'rose' });
  expect(hook.result.current.brand.savedBrand.name).toBe('');
  expect(hook.result.current.session.view.companies[0].name).toBe('Catalog X');
  await act(async () => {
    success = await hook.result.current.save!(draft);
  });
  expect(success).toBe(true);
  expect(hook.result.current.brand.savedBrand).toEqual(draft);
  expect(hook.result.current.session.view.companies[0].name).toBe('Catalog X');
  expect(access.saveBrand).toHaveBeenLastCalledWith('x', 'a', draft);
});
it('retries retained brand loading explicitly and keeps saving unavailable until a valid read', async () => {
  vi.mocked(access.getBrand)
    .mockResolvedValueOnce({ ...state, code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce({
      ...state,
      brand: { name: 'Loaded', logo: '', accent: 'blue' },
    });
  const hook = renderHook(useRetainedBrand);
  await waitFor(() =>
    expect(hook.result.current.brand.brandError).toContain('serviço'),
  );
  expect(hook.result.current.save).toBeUndefined();
  expect(access.getBrand).toHaveBeenCalledTimes(1);
  act(() => hook.result.current.brand.retryBrand((n) => n + 1));
  await waitFor(() =>
    expect(hook.result.current.brand.savedBrand.name).toBe('Loaded'),
  );
  expect(hook.result.current.save).toBeTypeOf('function');
  expect(access.getBrand).toHaveBeenCalledTimes(2);
  expect(access.saveBrand).not.toHaveBeenCalled();
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
it('reconciles retained brand data after navigation invalidates the original committed response', async () => {
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
  const hook = renderHook(useRetainedBrand);
  await waitFor(() => expect(hook.result.current.save).toBeTypeOf('function'));
  let saving!: Promise<boolean>;
  act(() => {
    saving = hook.result.current.save!({
      name: 'Saved later',
      logo: '',
      accent: 'blue',
    });
  });
  expect(access.saveBrand).toHaveBeenCalledOnce();
  act(() => {
    hook.result.current.session.invalidate();
    hook.result.current.session.bumpSurface();
  });
  await act(async () => {
    resolve({
      ...state,
      brand: { name: 'Discarded direct response', logo: '', accent: 'rose' },
    });
    await saving;
  });
  await waitFor(() =>
    expect(hook.result.current.brand.savedBrand.name).toBe('Saved later'),
  );
  expect(access.getBrand).toHaveBeenCalledTimes(2);
  expect(hook.result.current.brand.savedBrand.accent).toBe('blue');
  expect(hook.result.current.session.view.companies[0].name).toBe('Catalog X');
});
