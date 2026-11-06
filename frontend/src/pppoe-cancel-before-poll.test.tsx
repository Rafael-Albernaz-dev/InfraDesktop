import { act, render, screen, waitFor } from '@testing-library/react';
import { useLayoutEffect } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import { usePPReadState } from './pppoe-read-state';
import { pppoeReadAPI as api } from './presentation';
import type { Device } from './equipment-api';
vi.mock('./presentation', () => ({
  pppoeReadAPI: {
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
  },
}));
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function CancelOnFirstCommit() {
  const {
    start,
    cancel,
    job,
    result: reading,
  } = usePPReadState({
    profile: 'company',
    activation: 'active',
    observation: 'saved',
    rows: ['row'],
    device: { id: 'synthetic', revision: 1, active: true } as Device,
  });
  useLayoutEffect(() => {
    void start('bandwidth');
  }, [start]);
  useLayoutEffect(() => {
    if (job) void cancel();
  }, [job, cancel]);
  return <output>{reading ? 'resultado indevido' : 'sem resultado'}</output>;
}
afterEach(() => vi.resetAllMocks());
it('does not poll or publish when cancellation precedes the first polling effect', async () => {
  let finish!: (r: ReturnType<typeof result>) => void;
  vi.mocked(api.start).mockResolvedValue(
    result({ job: { id: 'synthetic-job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.cancel).mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  vi.mocked(api.status).mockResolvedValue(
    result({
      job: { id: 'synthetic-job', state: 'SUCCEEDED', code: 'OK' },
      read: { rows: [], scope: 'bandwidth' },
    }),
  );
  const view = render(<CancelOnFirstCommit />);
  try {
    await waitFor(() => expect(api.cancel).toHaveBeenCalled());
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByRole('status')).toHaveTextContent('sem resultado');
    await act(async () => finish(result()));
    expect(screen.getByRole('status')).toHaveTextContent('sem resultado');
    expect(api.status).not.toHaveBeenCalled();
  } finally {
    view.unmount();
  }
});
