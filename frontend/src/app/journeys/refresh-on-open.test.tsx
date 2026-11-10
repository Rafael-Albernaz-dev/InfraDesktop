import { latestRefreshAttempt } from '../../shared/hooks/refresh-attempt-order';
import { RadioCodes } from '../../features/radio/radio-contract';
import { StrictMode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import {
  shouldRefreshOnOpen,
  useRefreshOnOpen,
} from '../../features/radio/use-refresh-on-open';
import type { RefreshOnOpenInput } from '../../features/radio/use-refresh-on-open';
const now = Date.parse('2026-11-04T12:00:00Z');
const ago = (seconds: number) => new Date(now - seconds * 1000).toISOString();
const input = (): RefreshOnOpenInput => ({
  now,
  observedAt: ago(61),
  device: { active: true, fingerprint: 'synthetic', credential_id: 'bound' },
  vaultAvailable: true,
  serviceAvailable: true,
  reading: false,
});
const attempt = (code: string, seconds: number) => ({
  state: 'FAILED',
  code,
  at: ago(seconds),
});
describe('seis condições de leitura ao abrir', () => {
  it.each([
    ['idade 59 s', { observedAt: ago(59) }],
    ['idade 60 s', { observedAt: ago(60) }],
    ['sem leitura', { observedAt: undefined }],
    ['leitura em curso', { reading: true }],
    ['cofre indisponível', { vaultAvailable: false }],
    ['serviço indisponível', { serviceAvailable: false }],
  ])('recusa %s', (_name, change) =>
    expect(shouldRefreshOnOpen({ ...input(), ...change })).toBe(false),
  );
  it.each([
    ['inativo', { active: false }],
    ['sem chave', { fingerprint: '' }],
    ['sem credencial vinculada', { credential_id: '' }],
    [
      'credencial recusada há 6 min',
      { last_attempt: attempt(RadioCodes.REMOTE_AUTH_FAILED, 360) },
    ],
    ['falha há 4 min', { last_attempt: attempt('CONNECT_FAILED', 240) }],
    [
      'tentativa iniciada',
      { last_attempt: { ...attempt('OK', 0), state: 'STARTED' } },
    ],
  ])('recusa %s', (_name, change) =>
    expect(
      shouldRefreshOnOpen({
        ...input(),
        device: { ...input().device, ...change },
      }),
    ).toBe(false),
  );
  it('libera aos 61 s e depois de falha há 6 min, sem efeito colateral', () => {
    const p = input();
    const original = JSON.stringify(p);
    expect(shouldRefreshOnOpen(p)).toBe(true);
    expect(JSON.stringify(p)).toBe(original);
    expect(
      shouldRefreshOnOpen({
        ...p,
        device: { ...p.device, last_attempt: attempt('CONNECT_FAILED', 360) },
      }),
    ).toBe(true);
  });
  it('mantém a recusa até uma ação humana substituir a tentativa', () => {
    const p = input();
    p.device.last_attempt = attempt(RadioCodes.REMOTE_AUTH_FAILED, 3600);
    expect(shouldRefreshOnOpen(p)).toBe(false);
    p.device.last_attempt = { ...attempt('OK', 0), state: 'SUCCEEDED' };
    expect(shouldRefreshOnOpen(p)).toBe(true);
  });
});
it('decide uma vez sob StrictMode, reconciliação e troca de ativação', async () => {
  const refresh = vi.fn();
  const initial = {
    opening: 'router|Resumo',
    scope: 'company|active',
    ready: false,
    input: input(),
    refresh,
  };
  const hook = renderHook((p) => useRefreshOnOpen(p), {
    initialProps: initial,
    wrapper: StrictMode,
  });
  expect(refresh).not.toHaveBeenCalled();
  hook.rerender({ ...initial, ready: true });
  await act(async () => {});
  expect(refresh).toHaveBeenCalledOnce();
  hook.rerender({ ...initial, ready: true, input: { ...input() } });
  hook.rerender({ ...initial, ready: true, scope: 'company|new-activation' });
  expect(refresh).toHaveBeenCalledOnce();
  hook.rerender({ ...initial, opening: '', ready: true });
  hook.rerender({
    ...initial,
    ready: true,
    input: { ...input(), observedAt: ago(0) },
  });
  expect(refresh).toHaveBeenCalledOnce();
});
it('não reconecta quando uma condição muda depois da decisão da abertura', () => {
  const refresh = vi.fn();
  const initial = {
    opening: 'router|Sessões',
    scope: 'company|active',
    ready: true,
    input: { ...input(), vaultAvailable: false },
    refresh,
  };
  const hook = renderHook((p) => useRefreshOnOpen(p), {
    initialProps: initial,
  });
  hook.rerender({ ...initial, input: input() });
  expect(refresh).not.toHaveBeenCalled();
});

it('preserva a decisão quando a leitura remonta pela revisão do equipamento', () => {
  const refresh = vi.fn();
  const decision = {
    current: undefined,
  } as import('../../features/radio/use-refresh-on-open').RefreshOpeningRef;
  const props = {
    opening: 'router|Resumo',
    scope: 'company|active',
    ready: true,
    input: input(),
    refresh,
    decision,
  };
  const first = renderHook(() => useRefreshOnOpen(props));
  expect(refresh).toHaveBeenCalledOnce();
  first.unmount();
  renderHook(() => useRefreshOnOpen(props));
  expect(refresh).toHaveBeenCalledOnce();
});

it('uma observação PPPoE antiga bem sucedida não esconde recusa mais recente do equipamento', () => {
  const refused = attempt(RadioCodes.REMOTE_AUTH_FAILED, 60);
  const old = { ...attempt('OK', 3600), state: 'SUCCEEDED' };
  const p = input();
  p.device.last_attempt = latestRefreshAttempt(refused, old);
  expect(shouldRefreshOnOpen(p)).toBe(false);
  const manual = { ...attempt('OK', 0), state: 'SUCCEEDED' };
  p.device.last_attempt = latestRefreshAttempt(refused, manual);
  expect(shouldRefreshOnOpen(p)).toBe(true);
});
