import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from '../../features/pppoe/pppoe-diagnostics';
import { pppoeReadAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
const device = { id: 'router', revision: 1 } as Device;
const result = (data = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
const running = () =>
  result({ job: { id: 'job', state: 'RUNNING', code: 'OK' } });
const element = (blocked: boolean) => (
  <PPPoEReadPanel
    profile="company"
    activation="active"
    device={device}
    observation="saved"
    rows={['row']}
    traffic
    panel
    blocked={blocked}
  />
);
const advance = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
function visibility() {
  const doc = render(<div />).container.ownerDocument;
  let state: DocumentVisibilityState = 'visible';
  vi.spyOn(doc, 'visibilityState', 'get').mockImplementation(() => state);
  return async (next: DocumentVisibilityState) => {
    state = next;
    fireEvent(doc, new Event('visibilitychange'));
    await advance(10);
  };
}
async function hideAndShow(show: ReturnType<typeof visibility>) {
  await show('hidden');
  await show('visible');
}
beforeEach(() => {
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  vi.mocked(pppoeReadAPI.cancelOperation).mockResolvedValue(result());
  vi.mocked(pppoeReadAPI.stop).mockResolvedValue(result());
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.resetAllMocks();
});
it('does not start a blocked panel when the window returns', async () => {
  vi.useFakeTimers();
  const show = visibility();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(running());
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(running());
  const view = render(element(true));
  await advance(1);
  await hideAndShow(show);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(0);
  view.unmount();
});
it('starts by itself once the block ends after the window returned', async () => {
  vi.useFakeTimers();
  const show = visibility();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(running());
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(running());
  const view = render(element(true));
  await advance(1);
  await hideAndShow(show);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(0);
  view.rerender(element(false));
  await advance(10);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
  view.unmount();
});
it.each(['TRUST_REQUIRED', 'DEVICE_INACTIVE', 'UNAUTHENTICATED'])(
  'stays stopped after permanent %s when the window returns',
  async (code) => {
    vi.useFakeTimers();
    const show = visibility();
    vi.mocked(pppoeReadAPI.start).mockResolvedValue({ ...result(), code });
    const view = render(element(false));
    await advance(1);
    await hideAndShow(show);
    expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
    view.unmount();
  },
);
it('stays stopped after exhausting retries until the operator asks', async () => {
  vi.useFakeTimers();
  const show = visibility();
  vi.mocked(pppoeReadAPI.start).mockResolvedValue({
    ...result(),
    code: 'BUSY',
  });
  const view = render(element(false));
  await advance(60000);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(4);
  await hideAndShow(show);
  await hideAndShow(show);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(4);
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(running());
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(running());
  fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' }));
  await advance(1);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(5);
  view.unmount();
});
it('still resumes after a passing failure while retries remain', async () => {
  vi.useFakeTimers();
  const show = visibility();
  vi.mocked(pppoeReadAPI.start).mockResolvedValueOnce({
    ...result(),
    code: 'BUSY',
  });
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(running());
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(running());
  const view = render(element(false));
  await advance(1);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
  await hideAndShow(show);
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2);
  view.unmount();
});
