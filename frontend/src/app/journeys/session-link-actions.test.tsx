import type { PPRow } from '../../platform/wails/presentation';
import { createRef } from 'react';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { PPPoESessionDetail } from '../pages/pppoe-with-link';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import { link } from '../../features/radio/radio-test-fixtures';
import {
  RadioModes,
  RadioRoles,
} from '../../features/equipment/radio-contract';
import type { Device } from '../../features/equipment/equipment-api';
import type { PPObservation } from '../../platform/wails/pppoe-types';

vi.mock('../../platform/wails/presentation', async (original) => ({
  ...(await original<typeof import('../../platform/wails/presentation')>()),
  equipmentAPI: Object.fromEntries(
    'get,list'.split(',').map((key) => [key, vi.fn()]),
  ),
  radioAPI: Object.fromEntries(
    'link,current,confirmLink,start,status,cancel,cancelOperation,indexPage'
      .split(',')
      .map((key) => [key, vi.fn()]),
  ),
}));
const response = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, more: false, next_offset: 0, ...data },
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
} as PPObservation;
function renderLink(open = vi.fn()) {
  render(
    <PPPoESessionDetail
      selected={selected}
      current={current}
      detailHeadingRef={createRef()}
      detailTab="link"
      setDetailTab={() => {}}
      profile="company"
      activation="active"
      device={device}
      disabled={false}
      detailIntentRef={{ current: 0 }}
      detailTriggerRef={{ current: null }}
      select={() => {}}
      back={() => {}}
      openEquipment={open}
    />,
  );
  return open;
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(radioAPI.link).mockResolvedValue(response({ link }) as never);
  vi.mocked(radioAPI.current).mockResolvedValue(response() as never);
  vi.mocked(radioAPI.cancel).mockResolvedValue(response() as never);
  vi.mocked(radioAPI.cancelOperation).mockResolvedValue(response() as never);
  vi.mocked(equipmentAPI.get).mockImplementation(
    async (_p, _a, id) =>
      response({ device: { id, name: `Nome ${id}` } }) as never,
  );
  vi.mocked(equipmentAPI.list).mockResolvedValue(
    response({ devices: [], next_cursor: 0, vault_available: true }) as never,
  );
});
it('opens the linked equipment and gates live behind load testing from the session', async () => {
  vi.mocked(radioAPI.start).mockResolvedValue(
    response({
      job: { id: 'radio-job', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  vi.mocked(radioAPI.status).mockResolvedValue(
    response({
      job: { id: 'radio-job', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  const open = renderLink();
  const panel = await screen.findByRole('region', { name: 'Enlace rádio' });
  fireEvent.click(await within(panel).findByText(`Nome ${link.ap_id}`));
  expect(open).toHaveBeenCalledWith(link.ap_id);
  expect(
    within(panel).getByRole('button', { name: 'Ver ao vivo' }),
  ).toBeDisabled();
  fireEvent.click(within(panel).getByRole('button', { name: 'Atualizar' }));
  await waitFor(() => expect(radioAPI.start).toHaveBeenCalledOnce());
  expect(vi.mocked(radioAPI.start).mock.calls[0][2]).toMatchObject({
    device_id: 'router-fixture',
    observation_id: 'obs-fixture',
    row_id: 'row-fixture',
    mode: 'once',
  });
  fireEvent.click(await within(panel).findByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(radioAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'radio-job',
    ),
  );
  fireEvent.click(
    await within(panel).findByRole('button', {
      name: 'Testar carga do ao vivo',
    }),
  );
  await waitFor(() => expect(radioAPI.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(radioAPI.start).mock.calls[1][2]).toMatchObject({
    mode: 'measure',
  });
});
it('reopens an unavailable stored link from the session', async () => {
  vi.mocked(radioAPI.link)
    .mockResolvedValueOnce({
      ...response(),
      code: 'SERVICE_UNAVAILABLE',
    } as never)
    .mockResolvedValue(response({ link }) as never);
  renderLink();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reabrir vínculo armazenado' }),
  );
  await waitFor(() => expect(radioAPI.link).toHaveBeenCalledTimes(3));
  expect(
    await screen.findByRole('button', { name: 'Revisar vínculo' }),
  ).toBeVisible();
});

it('confirms a suggested link through the new session screen', async () => {
  vi.mocked(radioAPI.link)
    .mockResolvedValueOnce(response({ suggestions: [link] }) as never)
    .mockResolvedValueOnce(response({ suggestions: [link] }) as never)
    .mockResolvedValue(response({ link }) as never);
  vi.mocked(radioAPI.confirmLink).mockResolvedValue(
    response({ link }) as never,
  );
  renderLink();
  fireEvent.click(
    await screen.findByRole('button', { name: /Confirmar sugestão/ }),
  );
  await waitFor(() => expect(radioAPI.confirmLink).toHaveBeenCalledOnce());
  expect(vi.mocked(radioAPI.confirmLink).mock.calls[0].slice(0, 5)).toEqual([
    'company',
    'active',
    'router-fixture',
    'obs-fixture',
    'row-fixture',
  ]);
  expect(
    await screen.findByRole('button', { name: 'Revisar vínculo' }),
  ).toBeVisible();
});

it('enables live only after a successful load test and exposes tick receipts', async () => {
  vi.mocked(radioAPI.start).mockImplementation(
    async (_p, _a, input) =>
      response({
        job: { id: `job-${input.mode}`, state: 'RUNNING', code: 'OK' },
      }) as never,
  );
  vi.mocked(radioAPI.status).mockImplementation(
    async (_p, _a, id) =>
      response({
        link,
        cost: {
          enabled: true,
          samples: [],
          p95_seconds: 1,
          interval_seconds: 5,
          reason: 'OK',
          series: [
            {
              observed_at: '2026-11-06T00:00:00Z',
              cpu: [10],
              ap_command_seconds: 1,
              cpe_command_seconds: 1,
              icmp_rtt_ms: [2],
              reason: 'OK',
            },
          ],
        },
        job: {
          id,
          state: id === 'job-measure' ? 'SUCCEEDED' : 'RUNNING',
          code: 'OK',
        },
      }) as never,
  );
  renderLink();
  const panel = await screen.findByRole('region', { name: 'Enlace rádio' });
  fireEvent.click(
    await within(panel).findByRole('button', {
      name: 'Testar carga do ao vivo',
    }),
  );
  await waitFor(() =>
    expect(
      within(panel).getByRole('button', { name: 'Ver ao vivo' }),
    ).toBeEnabled(),
  );
  fireEvent.click(within(panel).getByRole('button', { name: 'Ver ao vivo' }));
  await waitFor(() => expect(radioAPI.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(radioAPI.start).mock.calls[1][2]).toMatchObject({
    mode: RadioModes.live,
  });
  expect(
    await within(panel).findByText('Série por tick (1)'),
  ).toBeInTheDocument();
  fireEvent.click(within(panel).getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(radioAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active',
      'job-live',
    ),
  );
});

it('confirms a manually selected AP and CPE from the session', async () => {
  vi.mocked(equipmentAPI.list).mockResolvedValue(
    response({
      devices: [
        {
          id: link.cpe_id,
          name: 'CPE sintética',
          address: '192.0.2.10',
          role: RadioRoles.cpe,
        },
        {
          id: link.ap_id,
          name: 'AP sintético',
          address: '192.0.2.11',
          role: RadioRoles.ap,
        },
      ],
      next_cursor: 0,
      vault_available: true,
    }) as never,
  );
  vi.mocked(radioAPI.confirmLink).mockResolvedValue(
    response({ link }) as never,
  );
  renderLink();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Revisar vínculo' }),
  );
  await screen.findByRole('option', { name: /CPE sintética/ });
  fireEvent.change(screen.getByLabelText('CPE do vínculo'), {
    target: { value: link.cpe_id },
  });
  fireEvent.change(screen.getByLabelText('AP do vínculo'), {
    target: { value: link.ap_id },
  });
  fireEvent.change(screen.getByLabelText('MAC wireless da CPE'), {
    target: { value: link.mac },
  });
  fireEvent.change(screen.getByLabelText('Interface do AP'), {
    target: { value: link.interface },
  });
  fireEvent.change(screen.getByLabelText('BSSID do AP'), {
    target: { value: link.bssid },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Confirmar vínculo' }));
  await waitFor(() => expect(radioAPI.confirmLink).toHaveBeenCalledOnce());
  expect(vi.mocked(radioAPI.confirmLink).mock.calls[0][6]).toMatchObject({
    ap_id: link.ap_id,
    cpe_id: link.cpe_id,
    mac: link.mac,
  });
});

it('refreshes the AP index from the session without starting live', async () => {
  vi.mocked(radioAPI.link).mockResolvedValue(
    response({
      index: {
        observed_at: '2026-11-06T00:00:00Z',
        complete: true,
        targets: {},
        total: 1,
        offset: 0,
        more: false,
      },
    }) as never,
  );
  vi.mocked(radioAPI.start).mockResolvedValue(
    response({
      job: { id: 'index-job', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  vi.mocked(radioAPI.status).mockResolvedValue(
    response({
      job: { id: 'index-job', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  renderLink();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Atualizar índice' }),
  );
  await waitFor(() => expect(radioAPI.start).toHaveBeenCalledOnce());
  expect(vi.mocked(radioAPI.start).mock.calls[0][2]).toMatchObject({
    mode: RadioModes.index,
    observation_id: 'obs-fixture',
    row_id: 'row-fixture',
  });
});

it('confirms an assisted link proposal from the session', async () => {
  const proposal = {
    ap_name: 'AP sintético',
    ap_address: '192.0.2.11',
    credential_alias: 'consulta',
    address: '192.0.2.10',
    family: 'synthetic',
    credential_id: 'credencial-fixture',
    choice: link,
  };
  vi.mocked(radioAPI.link)
    .mockResolvedValueOnce(response({ proposal }) as never)
    .mockResolvedValueOnce(response({ proposal }) as never)
    .mockResolvedValue(response({ link }) as never);
  vi.mocked(radioAPI.confirmLink).mockResolvedValue(
    response({ link }) as never,
  );
  renderLink();
  fireEvent.click(
    await screen.findByRole('button', { name: 'Confirmar enlace sugerido' }),
  );
  await waitFor(() => expect(radioAPI.confirmLink).toHaveBeenCalledOnce());
  expect(vi.mocked(radioAPI.confirmLink).mock.calls[0][6]).toMatchObject({
    ap_id: link.ap_id,
    cpe_id: link.cpe_id,
  });
});

it('shows a radio failure on the session and does not retry it', async () => {
  vi.mocked(radioAPI.start).mockResolvedValue(
    response({
      job: { id: 'failed-radio', state: 'RUNNING', code: 'OK' },
    }) as never,
  );
  vi.mocked(radioAPI.status).mockResolvedValue(
    response({
      link,
      job: { id: 'failed-radio', state: 'FAILED', code: 'PARTIAL' },
    }) as never,
  );
  renderLink();
  fireEvent.click(await screen.findByRole('button', { name: 'Atualizar' }));
  expect(await screen.findByRole('alert')).toBeVisible();
  expect(radioAPI.start).toHaveBeenCalledOnce();
});
