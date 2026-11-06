import { createRef } from 'react';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoESessionDetail } from './pppoe-session-screen';
import { pppoeDiagnosticAPI, pppoeReadAPI, radioAPI } from './presentation';
import type { Device } from './equipment-api';
import type { PPObservation, PPRow } from './presentation';

vi.mock('./presentation', async (original) => ({
  ...(await original<typeof import('./presentation')>()),
  pppoeDiagnosticAPI: Object.fromEntries(
    'current,list,ping,status,cancel'.split(',').map((key) => [key, vi.fn()]),
  ),
  pppoeReadAPI: Object.fromEntries(
    'start,status,cancel,cancelOperation,stop,page'
      .split(',')
      .map((key) => [key, vi.fn()]),
  ),
  radioAPI: Object.fromEntries(
    'link,current'.split(',').map((key) => [key, vi.fn()]),
  ),
}));
const response = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
const device = {
  id: 'router-fixture',
  name: 'Concentrador sintético',
  revision: 1,
  active: true,
  fingerprint: 'pinned',
} as Device;
const selected = {
  row_id: 'row-fixture',
  login: 'usuario sintético',
  state: 'ONLINE',
  address: '192.0.2.10',
  uptime: { known: true, min: 1, max: 1 },
} as PPRow;
const current = {
  observation_id: 'obs-fixture',
  provenance: { ended_at: '2026-11-06T00:00:00Z' },
  context_omitted: true,
} as PPObservation;
function renderInformation() {
  render(
    <PPPoESessionDetail
      selected={selected}
      current={current}
      detailHeadingRef={createRef()}
      detailTab="information"
      setDetailTab={() => {}}
      profile="company"
      activation="active"
      device={device}
      disabled={false}
      detailIntentRef={{ current: 0 }}
      detailTriggerRef={{ current: null }}
      select={() => {}}
      back={() => {}}
    />,
  );
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(radioAPI.link).mockResolvedValue(response() as never);
  vi.mocked(pppoeDiagnosticAPI.current).mockResolvedValue(response() as never);
  vi.mocked(pppoeDiagnosticAPI.list).mockResolvedValue(response() as never);
  vi.mocked(pppoeDiagnosticAPI.cancel).mockResolvedValue(response() as never);
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(response() as never);
  vi.mocked(pppoeReadAPI.cancelOperation).mockResolvedValue(
    response() as never,
  );
});
it('reopens the stored ping, checks uncertain progress and cancels the exact session job', async () => {
  vi.mocked(pppoeDiagnosticAPI.ping).mockResolvedValue(
    response({
      job: { id: 'ping-fixture', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  vi.mocked(pppoeDiagnosticAPI.status)
    .mockRejectedValueOnce(new Error('synthetic status'))
    .mockResolvedValue(
      response({
        job: { id: 'ping-fixture', state: 'RUNNING', code: 'OK' },
      }) as never,
    );
  renderInformation();
  const panel = await screen.findByRole('region', {
    name: 'Sondagem da sessão pelo concentrador',
  });
  fireEvent.click(
    await within(panel).findByRole('button', {
      name: 'Reabrir sondagem armazenada',
    }),
  );
  await waitFor(() =>
    expect(pppoeDiagnosticAPI.current).toHaveBeenCalledTimes(2),
  );
  fireEvent.click(await within(panel).findByRole('button', { name: 'Ping' }));
  await within(panel).findByRole('button', { name: 'Conferir andamento' });
  fireEvent.click(
    within(panel).getByRole('button', { name: 'Conferir andamento' }),
  );
  await waitFor(() =>
    expect(pppoeDiagnosticAPI.status).toHaveBeenCalledTimes(2),
  );
  fireEvent.click(
    within(panel).getByRole('button', { name: 'Cancelar sondagem' }),
  );
  await waitFor(() =>
    expect(pppoeDiagnosticAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'ping-fixture',
    ),
  );
  expect(pppoeDiagnosticAPI.ping).toHaveBeenCalledWith(
    'company',
    'active',
    'router-fixture',
    'obs-fixture',
    'row-fixture',
    expect.any(String),
    1,
  );
});
it('starts log and secret evidence from the new session screen for its selected row', async () => {
  vi.mocked(pppoeReadAPI.start).mockImplementation(
    async (...args) =>
      response({
        job: { id: `job-${args[4]}`, state: 'RUNNING', code: 'OK' },
      }) as never,
  );
  vi.mocked(pppoeReadAPI.status).mockImplementation(
    async (_p, _a, id) =>
      response({ job: { id, state: 'RUNNING', code: 'OK' } }) as never,
  );
  renderInformation();
  fireEvent.click(await screen.findByRole('button', { name: 'Log PPP' }));
  fireEvent.click(
    screen.getByRole('button', { name: 'Ver evidência do secret' }),
  );
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2));
  const calls = vi.mocked(pppoeReadAPI.start).mock.calls;
  expect(calls.map((args) => args.slice(4, 8))).toEqual([
    ['log', 'obs-fixture', ['row-fixture'], 1],
    ['secret', 'obs-fixture', ['row-fixture'], 1],
  ]);
});
