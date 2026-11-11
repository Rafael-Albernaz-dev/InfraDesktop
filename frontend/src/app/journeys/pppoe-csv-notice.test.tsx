import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoE } from '../pages/pppoe-with-link';
import {
  pppoeAPI as api,
  pppoeReadAPI as read,
  deviceMonitoringAPI as monitoring,
  pppoeDiagnosticAPI as diagnostic,
} from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
import type { PPResult } from '../../platform/wails/presentation';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
const response = (data: PPResult['data'] = { total: 0 }): PPResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data,
});
const device = {
  id: 'synthetic',
  name: 'Synthetic',
  revision: 1,
  active: true,
  fingerprint: 'pinned',
} as Device;
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(monitoring.current).mockResolvedValue(response());
  vi.mocked(diagnostic.current).mockResolvedValue(response());
  vi.mocked(diagnostic.list).mockResolvedValue(response());
  vi.mocked(read.cancel).mockResolvedValue(response());
  vi.mocked(read.cancelOperation).mockResolvedValue(response());
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.references).mockResolvedValue(response());
  vi.mocked(api.groups).mockResolvedValue(response());
  vi.mocked(api.list).mockResolvedValue(response());
});
it('keeps the CSV privacy notice visible when no reading exists', async () => {
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await waitFor(() =>
    expect(
      screen.getByText(
        'O CSV contém dados identificáveis de assinantes. O JSON sanitizado não contém.',
      ),
    ).toBeVisible(),
  );
});
it('keeps the same notice visible alongside a saved reading', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      total: 0,
      current: {
        observation_id: 'synthetic',
        count: 0,
        provenance: { ended_at: '2026-11-05T10:00:00Z' },
        quality: { status: 'complete' },
      },
    }),
  );
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={vi.fn()}
    />,
  );
  await waitFor(() =>
    expect(
      screen.getByText(
        'O CSV contém dados identificáveis de assinantes. O JSON sanitizado não contém.',
      ),
    ).toBeVisible(),
  );
});
