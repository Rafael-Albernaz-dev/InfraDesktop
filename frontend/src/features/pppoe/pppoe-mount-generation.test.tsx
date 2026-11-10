import { render, screen, waitFor } from '@testing-library/react';
import { useLayoutEffect } from 'react';
import { expect, it, vi } from 'vitest';
import { usePPReadState } from './pppoe-read-state';

const calls = vi.hoisted(() => ({
  start: vi.fn(),
  status: vi.fn(),
  cancel: vi.fn(),
  cancelOperation: vi.fn(),
}));
vi.mock('../../platform/wails/presentation', () => ({ pppoeReadAPI: calls }));

function EarlyStart() {
  const { start, result, pending } = usePPReadState({
    profile: 'fixture',
    activation: 'activation',
    observation: 'observation',
    rows: ['row'],
    device: { id: 'synthetic', revision: 1 } as never,
  });
  useLayoutEffect(() => {
    void start('bandwidth');
  }, [start]);
  return (
    <output>
      {result ? 'resultado' : pending ? 'pendente' : 'sem resultado'}
    </output>
  );
}

it('does not invalidate a start issued before passive mount effects', async () => {
  const result = (data: unknown) => ({
    code: 'OK',
    profile_id: 'fixture',
    activation_id: 'activation',
    data,
  });
  calls.start.mockResolvedValue(
    result({ job: { id: 'job', state: 'RUNNING', code: 'OK' } }),
  );
  calls.status.mockResolvedValue(
    result({
      job: { id: 'job', state: 'SUCCEEDED', code: 'OK' },
      read: { ended_at: 'fixture' },
    }),
  );
  calls.cancel.mockResolvedValue(result({}));
  calls.cancelOperation.mockResolvedValue(result({}));
  render(<EarlyStart />);
  await waitFor(() => expect(calls.status).toHaveBeenCalledOnce());
  expect(await screen.findByText('resultado')).toBeInTheDocument();
  expect(calls.cancel).not.toHaveBeenCalled();
});
