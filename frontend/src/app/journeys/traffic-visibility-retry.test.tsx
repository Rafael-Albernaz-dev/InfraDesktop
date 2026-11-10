import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from '../../features/pppoe/pppoe-diagnostics';
import { pppoeReadAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', () => ({
  pppoeReadAPI: {
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    stop: vi.fn(),
    page: vi.fn(),
  },
  pppoeDiagnosticAPI: {},
  deviceMonitoringAPI: {},
  pppoeAPI: {},
}));
const result = (data = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function panel() {
  return render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={{ id: 'router', revision: 1 } as Device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
}
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.resetAllMocks();
});
it('cancels while hidden, resumes on visibility and respects an operator stop', async () => {
  vi.useFakeTimers();
  const doc = render(<div />).container.ownerDocument;
  let visibility: DocumentVisibilityState = 'visible';
  vi.spyOn(doc, 'visibilityState', 'get').mockImplementation(() => visibility);
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    result({ job: { id: 'traffic', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    result({ job: { id: 'traffic', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  vi.mocked(pppoeReadAPI.stop).mockResolvedValue(result());
  const view = panel();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  visibility = 'hidden';
  fireEvent(doc, new Event('visibilitychange'));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(60000);
  });
  expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
    'company',
    'active',
    'traffic',
  );
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1);
  visibility = 'visible';
  fireEvent(doc, new Event('visibilitychange'));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  visibility = 'hidden';
  fireEvent(doc, new Event('visibilitychange'));
  visibility = 'visible';
  fireEvent(doc, new Event('visibilitychange'));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(120000);
  });
  expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2);
  view.unmount();
});
it.each(['BUSY', 'DEADLINE_EXCEEDED', 'SERVICE_UNAVAILABLE'])(
  'retries %s only after 5, 15 and 30 seconds',
  async (code) => {
    vi.useFakeTimers();
    vi.mocked(pppoeReadAPI.start).mockResolvedValue({ ...result(), code });
    vi.mocked(pppoeReadAPI.cancelOperation).mockResolvedValue(result());
    const view = panel();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    for (const [delay, count] of [
      [4998, 1],
      [1, 2],
      [14999, 2],
      [1, 3],
      [29999, 3],
      [1, 4],
      [120000, 4],
    ]) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(delay);
      });
      expect(pppoeReadAPI.start).toHaveBeenCalledTimes(count);
    }
    expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeEnabled();
    view.unmount();
  },
);
