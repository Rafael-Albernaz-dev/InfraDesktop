import { act, fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { SessionCPE } from '../../features/pppoe/session-cpe';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
import type { RadioResult } from '../../features/equipment/radio-api';
vi.mock('../../platform/wails/presentation', () => ({
  radioAPI: { link: vi.fn() },
  equipmentAPI: { get: vi.fn() },
}));
it('mostra nome pelo vínculo local e descarta resposta de uma linha anterior sem abrir equipamento', async () => {
  const result = (data = {}): RadioResult => ({
    code: 'OK',
    profile_id: 'p',
    activation_id: 'a',
    data: { total: 0, more: false, next_offset: 0, ...data },
  });
  vi.mocked(radioAPI.link).mockResolvedValueOnce(
    result({ link: { cpe_id: 'opaque' } }),
  );
  vi.mocked(equipmentAPI.get).mockResolvedValue({
    code: 'OK',
    profile_id: 'p',
    activation_id: 'a',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      device: { id: 'opaque', name: 'CPE sintética' } as Device,
    },
  });
  const open = vi.fn();
  const props = {
    profile: 'p',
    activation: 'a',
    deviceId: 'd',
    observation: 'o',
    row: 'one',
    open,
  };
  const view = render(<SessionCPE {...props} />);
  const control = await screen.findByRole('button', { name: 'CPE sintética' });
  expect(open).not.toHaveBeenCalled();
  fireEvent.click(control);
  expect(open).toHaveBeenCalledExactlyOnceWith('opaque');
  let resolve!: (r: RadioResult) => void;
  vi.mocked(radioAPI.link)
    .mockReturnValueOnce(
      new Promise((done) => {
        resolve = done;
      }),
    )
    .mockResolvedValueOnce(result());
  view.rerender(<SessionCPE {...props} row="old" />);
  await act(async () => {});
  view.rerender(<SessionCPE {...props} row="new" />);
  await screen.findByText('Sem vínculo');
  await act(async () => resolve(result({ link: { cpe_id: 'late' } })));
  expect(equipmentAPI.get).toHaveBeenCalledOnce();
  expect(screen.getByText('Sem vínculo')).toBeVisible();
  expect(open).toHaveBeenCalledOnce();
});
