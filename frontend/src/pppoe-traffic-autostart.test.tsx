import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { pppoeReadAPI } from './presentation';
import type { Device } from './equipment-api';

vi.mock('./presentation', () => ({
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
const device = {
  id: 'router',
  name: 'Sintético',
  revision: 1,
  active: true,
} as Device;
const result = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
afterEach(() => {
  vi.resetAllMocks();
});

it('starts session traffic on opening and cancels it on leaving', async () => {
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    result({ job: { id: 'traffic', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    result({ job: { id: 'traffic', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(result());
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
    />,
  );
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledTimes(1));
  expect(
    screen.queryByRole('button', { name: 'Ver ao vivo' }),
  ).not.toBeInTheDocument();
  expect(pppoeReadAPI.start).toHaveBeenCalledWith(
    'company',
    'active',
    'router',
    expect.any(String),
    'traffic',
    'saved',
    ['row'],
    1,
  );
  view.unmount();
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'traffic',
    ),
  );
});

it('does not start traffic while blocked', async () => {
  const view = render(
    <PPPoEReadPanel
      profile="company"
      activation="active"
      device={device}
      observation="saved"
      rows={['row']}
      traffic
      panel
      blocked
    />,
  );
  await Promise.resolve();
  expect(pppoeReadAPI.start).not.toHaveBeenCalled();
  view.unmount();
});
