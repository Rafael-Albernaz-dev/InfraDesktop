import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PPPoE } from '../pages/pppoe-with-link';
import type { Device } from '../../features/equipment/equipment-api';
import type { PPQuery } from '../../platform/wails/presentation-pppoe-types';
const device = {
  id: 'synthetic-router',
  name: 'Concentrador sintético',
  active: true,
  fingerprint: 'saved',
  revision: 1,
} as Device;
afterEach(() => {
  cleanup();
  vi.stubGlobal('go', undefined);
});
it('troca Estado e ordenação com cursor reiniciado e Limpar filtros recupera Todos e o foco', async () => {
  const result = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: { total: 2, ...data },
  });
  const list = vi.fn(
    async (...args: [string, string, string, string, PPQuery, string]) => {
      const query = args[4],
        cursor = args[5];
      const offline = query.state === 'OFFLINE' || Boolean(cursor);
      return result({
        rows: [
          {
            row_id: offline ? 'offline' : 'online',
            login: offline ? 'cliente-offline' : 'cliente-online',
            state: offline ? 'OFFLINE' : 'ONLINE',
            address: '192.0.2.100',
            uptime: { known: false, min: 0, max: 0 },
          },
        ],
        next_cursor: cursor || query.state ? '' : 'second',
      });
    },
  );
  vi.stubGlobal('go', {
    presentation: {
      App: {
        GetPPPoECurrent: async () =>
          result({
            current: {
              observation_id: 'saved',
              count: 2,
              provenance: { ended_at: '2026-11-04T12:00:00Z' },
              quality: { status: 'complete' },
            },
          }),
        ListPPPoESessions: list,
        ListPPPoEReferences: async () => result({ references: [] }),
        PPPoEContextCounts: async () => result({ groups: [] }),
        GetDeviceMonitoring: async () => result(),
        GetRadioLink: async () => result(),
      },
    },
  });
  render(
    <PPPoE
      profile="company"
      activation="active"
      device={device}
      back={() => {}}
    />,
  );
  await screen.findByRole('cell', { name: 'cliente-online' });
  fireEvent.click(screen.getByRole('button', { name: 'Próxima página' }));
  await screen.findByRole('cell', { name: 'cliente-offline' });
  expect(list.mock.calls.at(-1)?.[5]).toBe('second');
  fireEvent.change(screen.getByRole('combobox', { name: 'Estado' }), {
    target: { value: 'OFFLINE' },
  });
  await waitFor(() =>
    expect(list.mock.calls.at(-1)?.slice(4)).toEqual([
      expect.objectContaining({ state: 'OFFLINE' }),
      '',
    ]),
  );
  fireEvent.change(screen.getByRole('combobox', { name: 'Ordenar' }), {
    target: { value: 'state' },
  });
  await waitFor(() =>
    expect(list.mock.calls.at(-1)?.[4]).toMatchObject({
      state: 'OFFLINE',
      sort: 'state',
    }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  await screen.findByRole('cell', { name: 'cliente-online' });
  expect(screen.getByRole('combobox', { name: 'Estado' })).toHaveValue('');
  expect(screen.getByRole('combobox', { name: 'Ordenar' })).toHaveValue(
    'login',
  );
  expect(screen.getByRole('textbox', { name: 'Buscar sessões' })).toHaveFocus();
});
