import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { NetworkPulse } from './network-pulse-context';
import { useContext, useEffect } from 'react';
import { NetworkBadge, NetworkPulseProvider } from './network-pulse';
import { equipmentAPI } from './equipment-api';
import type { EquipmentResult } from './equipment-api';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
function result(pulse: string): EquipmentResult {
  return {
    code: 'OK',
    profile_id: 'p',
    activation_id: 'a',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: false,
      pulse_network: pulse,
    },
  };
}
let testDoc: Document;
const flush = () =>
  act(async () => {
    await Promise.resolve();
  });
function Badge(p: { connected?: boolean; profile?: string }) {
  return (
    <NetworkPulseProvider
      profile={p.profile || 'p'}
      activation="a"
      connected={p.connected !== false}
      doc={testDoc}
    >
      <NetworkBadge connected={p.connected !== false} />
    </NetworkPulseProvider>
  );
}
beforeEach(() => {
  testDoc = render(<div />).container.ownerDocument;
  vi.useFakeTimers();
  vi.clearAllMocks();
  vi.spyOn(testDoc, 'visibilityState', 'get').mockReturnValue('visible');
  vi.mocked(equipmentAPI.list).mockResolvedValue(result('ok'));
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});
it('renews the shell badge after five seconds and service loss overrides the last network state', async () => {
  const view = render(<Badge />);
  await flush();
  expect(screen.getByText('Conectado à rede')).toBeVisible();
  vi.mocked(equipmentAPI.list).mockResolvedValue(result('no_response'));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(4999);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(1);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(screen.getByText('Sem acesso à rede')).toBeVisible();
  view.rerender(<Badge connected={false} />);
  expect(screen.getByText('Serviço parado')).toBeVisible();
  expect(screen.queryByText('Conectado à rede')).not.toBeInTheDocument();
});
it('does not poll hidden windows or accept a late reading from the previous company', async () => {
  let finish: (r: EquipmentResult) => void = () => {};
  vi.mocked(equipmentAPI.list).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const view = render(<Badge />);
  vi.mocked(equipmentAPI.list).mockResolvedValue(result('probe_blocked'));
  view.rerender(<Badge profile="other" />);
  await flush();
  expect(screen.getByText('Ping sem permissão')).toBeVisible();
  await act(async () => {
    finish(result('ok'));
  });
  expect(screen.getByText('Ping sem permissão')).toBeVisible();
  vi.spyOn(testDoc, 'visibilityState', 'get').mockReturnValue('hidden');
  fireEvent(testDoc, new Event('visibilitychange'));
  const calls = vi.mocked(equipmentAPI.list).mock.calls.length;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(10000);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(calls);
});
function ListReader() {
  const { register, publish } = useContext(NetworkPulse);
  useEffect(register, [register]);
  return (
    <button onClick={() => publish('no_response')}>Leitura da lista</button>
  );
}
it('reuses the mounted list reader instead of starting a second polling loop', async () => {
  render(
    <NetworkPulseProvider profile="p" activation="a" connected doc={testDoc}>
      <ListReader />
      <NetworkBadge connected />
    </NetworkPulseProvider>,
  );
  await flush();
  fireEvent.click(screen.getByText('Leitura da lista'));
  expect(screen.getByText('Sem acesso à rede')).toBeVisible();
  const calls = vi.mocked(equipmentAPI.list).mock.calls.length;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(15000);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(calls);
});
