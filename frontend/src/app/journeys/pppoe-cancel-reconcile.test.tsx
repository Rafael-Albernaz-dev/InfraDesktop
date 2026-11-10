import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { usePPReadState } from '../../features/pppoe/pppoe-read-state';
import { pppoeReadAPI as api } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', () => ({
  pppoeReadAPI: {
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    stop: vi.fn(),
  },
}));
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function status(traffic: boolean, ended: string) {
  return result({
    job: { id: 'synthetic-job', state: 'SUCCEEDED', code: 'OK' },
    read: {
      started_at: '2026-11-06T12:00:00Z',
      ended_at: ended,
      duration_ns: 2e9,
      rows: [],
      scope: traffic ? 'traffic' : 'bandwidth',
    },
  });
}
function Reading({ traffic }: { traffic: boolean }) {
  const {
    start,
    cancel,
    retryPoll,
    cancelling,
    result: reading,
    error,
    setRoot,
    stoppedTraffic,
  } = usePPReadState({
    profile: 'company',
    activation: 'active',
    observation: 'saved',
    rows: ['row'],
    traffic,
    device: { id: 'synthetic', revision: 1, active: true } as Device,
  });
  return (
    <div ref={setRoot}>
      <button onClick={() => void start('bandwidth')}>Medir</button>
      <button disabled={cancelling} onClick={() => void cancel()}>
        Cancelar
      </button>
      <button disabled={cancelling} onClick={() => retryPoll((n) => n + 1)}>
        Conferir andamento
      </button>
      <output aria-label="Resultado">
        {reading?.ended_at || error || 'sem resultado'}
      </output>
      <output aria-label="Observação parada">{String(stoppedTraffic)}</output>
    </div>
  );
}
afterEach(() => vi.resetAllMocks());
it.each([false, true])(
  'reconciles unconfirmed cancellation (traffic=%s), preserving operator stop and ignoring old status',
  async (traffic) => {
    let oldStatus!: (r: ReturnType<typeof result>) => void;
    const fresh = '2026-11-06T12:00:02Z',
      obsolete = '2026-11-06T12:00:01Z';
    vi.mocked(api.start).mockResolvedValue(
      result({ job: { id: 'synthetic-job', state: 'RUNNING', code: 'OK' } }),
    );
    vi.mocked(api.status)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            oldStatus = resolve;
          }),
      )
      .mockResolvedValue(status(traffic, fresh));
    vi.mocked(api.cancel).mockRejectedValue(
      new Error('synthetic cancel transport loss'),
    );
    vi.mocked(api.stop).mockRejectedValue(
      new Error('synthetic stop transport loss'),
    );
    const view = render(<Reading traffic={traffic} />);
    try {
      if (!traffic)
        fireEvent.click(screen.getByRole('button', { name: 'Medir' }));
      await waitFor(() => expect(api.status).toHaveBeenCalledOnce());
      fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
      await screen.findByText(
        'Cancelamento não confirmado; o prazo da leitura continua limitado.',
      );
      fireEvent.click(
        screen.getByRole('button', { name: 'Conferir andamento' }),
      );
      expect(await screen.findByText(fresh)).toBeVisible();
      expect(api.start).toHaveBeenCalledOnce();
      expect(api.status).toHaveBeenLastCalledWith(
        'company',
        'active',
        'synthetic-job',
      );
      await act(async () => oldStatus(status(traffic, obsolete)));
      expect(screen.getByLabelText('Resultado')).toHaveTextContent(fresh);
      expect(screen.queryByText(obsolete)).not.toBeInTheDocument();
      if (traffic)
        expect(screen.getByLabelText('Observação parada')).toHaveTextContent(
          'true',
        );
    } finally {
      view.unmount();
    }
  },
);
