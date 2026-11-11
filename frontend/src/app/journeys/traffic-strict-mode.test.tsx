import { StrictMode } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { usePPReadState } from '../../features/pppoe/pppoe-read-state';
import { pppoeReadAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
function Traffic() {
  const { setRoot, job } = usePPReadState({
    profile: 'company',
    activation: 'active',
    device: { id: 'synthetic', revision: 1, active: true } as Device,
    observation: 'saved',
    rows: ['saved-row'],
    traffic: true,
    panel: true,
  });
  return (
    <div ref={setRoot}>
      <output aria-label="Janela ativa">{job || 'Nenhuma'}</output>
    </div>
  );
}
afterEach(() => vi.resetAllMocks());
it('restarts automatic traffic after StrictMode cleans up the first mount', async () => {
  let starts = 0;
  vi.mocked(pppoeReadAPI.start).mockImplementation(async () =>
    result({
      job: { id: `synthetic-${++starts}`, state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(pppoeReadAPI.status).mockImplementation(async (_p, _a, id) =>
    result({
      job: { id, state: 'RUNNING', code: 'OK' },
    }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  vi.mocked(pppoeReadAPI.cancelOperation).mockResolvedValue(result());
  const view = render(
    <StrictMode>
      <Traffic />
    </StrictMode>,
  );
  try {
    await waitFor(() =>
      expect(screen.getByLabelText('Janela ativa')).toHaveTextContent(
        'synthetic-2',
      ),
    );
    expect(pppoeReadAPI.status).toHaveBeenCalledWith(
      'company',
      'active',
      'synthetic-2',
    );
    expect(pppoeReadAPI.cancelOperation).toHaveBeenCalledTimes(1);
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'synthetic-1',
    );
  } finally {
    view.unmount();
  }
  expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
    'company',
    'active',
    'synthetic-2',
  );
});
