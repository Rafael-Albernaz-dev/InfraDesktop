import { act, render, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { RadioLinkPanel } from './radio';
import { radioAPI as api } from './radio-api';
import type { RadioResult } from './radio-api';
import { device, response } from './radio-test-fixtures';
import { equipmentAPI } from './equipment-api';
vi.mock('./radio-api', () => ({
  radioAPI: {
    link: vi.fn(),
    current: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    confirmLink: vi.fn(),
    confirmFamily: vi.fn(),
  },
}));
vi.mock('./equipment-api', () => ({
  equipmentMessage: (code: string) => code,
  equipmentAPI: {
    list: vi.fn(),
    get: vi.fn(),
    credentials: vi.fn(),
    save: vi.fn(),
  },
}));
it('does not begin an index when the view becomes blocked while its saved link is loading', async () => {
  let resolve!: (r: RadioResult) => void;
  vi.mocked(api.link).mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'unexpected', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({ job: { id: 'unexpected', state: 'SUCCEEDED', code: 'OK' } }),
  );
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      total_devices: 0,
      vault_available: true,
    },
  });
  const props = {
    profile: 'company',
    activation: 'active',
    device,
    observation: 'synthetic',
    row: 'row',
    pppoeUptime: '2 h',
  };
  const view = render(<RadioLinkPanel {...props} blocked={false} />);
  await waitFor(() => expect(api.link).toHaveBeenCalledOnce());
  view.rerender(<RadioLinkPanel {...props} blocked />);
  await act(async () => resolve(response({ match_reason: 'missing' })));
  expect(api.start).not.toHaveBeenCalled();
  view.unmount();
});
