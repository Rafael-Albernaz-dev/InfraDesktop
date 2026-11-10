import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import App from '../App';
import {
  access,
  equipmentAPI,
  pppoeAPI,
  pppoeDiagnosticAPI,
  pppoeReadAPI,
  radioAPI,
} from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
import type {
  PPResult,
  Result,
  PPRow,
} from '../../platform/wails/presentation';

import {
  RadioRoles,
  RadioTransports,
} from '../../features/equipment/radio-contract';
import {
  link as syntheticLink,
  reading as syntheticReading,
} from '../../features/radio/radio-test-fixtures';

vi.mock('../../platform/wails/presentation', async (original) => ({
  ...(await original<typeof import('../../platform/wails/presentation')>()),
  access: Object.fromEntries(
    'status,login,selectCompany,getBrand,logout'
      .split(',')
      .map((k) => [k, vi.fn()]),
  ),
  equipmentAPI: Object.fromEntries(
    'list,credentials,get,inspect'.split(',').map((k) => [k, vi.fn()]),
  ),
  pppoeAPI: Object.fromEntries(
    'groups,current,references,list,start,status,cancel,detail,save,remove,compare,export,exportOperational'
      .split(',')
      .map((k) => [k, vi.fn()]),
  ),
  pppoeReadAPI: Object.fromEntries(
    'stop,start,status,cancel,cancelOperation,page'
      .split(',')
      .map((k) => [k, vi.fn()]),
  ),
  pppoeDiagnosticAPI: Object.fromEntries(
    'current,list,catalog,radius,ping,status,cancel'
      .split(',')
      .map((k) => [k, vi.fn()]),
  ),
  radioAPI: Object.fromEntries(
    'link,current,profiles,apIndex,confirmLink,indexPage'
      .split(',')
      .map((k) => [k, vi.fn()]),
  ),
  deviceMonitoringAPI: { current: vi.fn() },
}));
const device = {
  id: 'ccr-fixture',
  name: 'CCR fixture',
  role: 'concentrator',
  address: '192.0.2.1',
  port: 22,
  transport: RadioTransports.ssh,
  revision: 1,
  active: true,
  fingerprint: 'pinned',
} as Device;
const rows: PPRow[] = ['primeiro', 'segundo'].map((login, i) => ({
  row_id: `row-${i}`,
  remote_id: `*${i}`,
  session_id: `${i}`,
  login,
  address: `192.0.2.${i + 20}`,
  caller_id: '02:00:00:00:00:01',
  uptime: { known: true, min: 1e9, max: 1e9 },
  duplicates: 0,
}));
const account = (company = 'company'): Result => ({
  code: 'OK',
  request_id: 'fixture',
  instance_id: 'one',
  activation_id: company === 'company-two' ? 'activation-two' : 'activation',
  profile_id: company,
  heartbeat: 0,
  setup: false,
  authenticated: true,
  name: 'fixture',
  theme: 'light',
  recovery: '',
  companies: [
    { id: 'company', name: 'Empresa um' },
    { id: 'company-two', name: 'Empresa dois' },
  ],
});
const pp = (data: PPResult['data'] = { total: 0 }): PPResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'activation',
  data,
});
const equip = {
  code: 'OK',
  profile_id: 'company',
  activation_id: 'activation',
  data: {
    devices: [device],
    device,
    credentials: [],
    next_cursor: 0,
    total_devices: 1,
    vault_available: true,
  },
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.status).mockResolvedValue(account());
  vi.mocked(access.selectCompany).mockImplementation(async (id) => account(id));
  vi.mocked(access.getBrand).mockResolvedValue(account());
  vi.mocked(equipmentAPI.list).mockResolvedValue(equip);
  vi.mocked(equipmentAPI.get).mockResolvedValue(equip);
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(equip);
  vi.mocked(pppoeAPI.current).mockResolvedValue(
    pp({
      current: {
        observation_id: 'obs',
        count: 2,
        context_omitted: true,
        provenance: { ended_at: '2026-11-06T00:00:00Z' },
        quality: { status: 'complete' },
      },
      total: 2,
    }),
  );
  vi.mocked(pppoeAPI.references).mockResolvedValue(
    pp({ references: [], total: 0 }),
  );
  vi.mocked(pppoeAPI.list).mockImplementation(
    async (_p, _a, _d, _o, _q, cursor) =>
      pp({
        rows: cursor ? [rows[1]] : [rows[0]],
        total: 2,
        next_cursor: cursor ? '' : 'next',
      }),
  );
  vi.mocked(pppoeAPI.detail).mockImplementation(async (_p, _a, _d, _o, row) =>
    pp({ rows: rows.filter((r) => r.row_id === row), total: 1 }),
  );
  vi.mocked(pppoeReadAPI.cancel).mockResolvedValue(pp());
  vi.mocked(pppoeDiagnosticAPI.current).mockResolvedValue(pp());
  vi.mocked(pppoeDiagnosticAPI.list).mockResolvedValue(pp());
  vi.mocked(pppoeReadAPI.cancelOperation).mockResolvedValue(pp());
  vi.mocked(radioAPI.link).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: { total: 0, more: false, next_offset: 0 },
  });
});
it('opens a linked session through the shell and returns to its origin', async () => {
  const cpe = {
    ...device,
    id: 'synthetic-cpe',
    name: 'CPE sintética',
    role: RadioRoles.cpe,
  };
  const concentrator = {
    ...device,
    id: 'concentrator',
    name: 'Concentrador sintético',
  };
  const row = { ...rows[0], row_id: 'linked-row', login: 'X' };
  const linked = {
    ...syntheticLink,
    cpe_id: cpe.id,
    concentrator_id: concentrator.id,
    login: 'X',
  };
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    ...equip,
    data: { ...equip.data, devices: [cpe, concentrator] },
  });
  vi.mocked(equipmentAPI.get).mockImplementation(async (_p, _a, id) => ({
    ...equip,
    data: { ...equip.data, device: id === cpe.id ? cpe : concentrator },
  }));
  vi.mocked(radioAPI.current).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: {
      reading: { ...syntheticReading, observed_at: new Date().toISOString() },
      link: linked,
      total: 0,
      more: false,
      next_offset: 0,
    },
  });
  vi.mocked(pppoeAPI.current).mockResolvedValue(
    pp({
      current: {
        observation_id: 'obs',
        count: 1,
        context_omitted: true,
        provenance: { ended_at: new Date().toISOString() },
        quality: { status: 'complete' },
      },
      total: 1,
    }),
  );
  vi.mocked(pppoeAPI.list).mockResolvedValue(pp({ rows: [row], total: 1 }));
  vi.mocked(pppoeAPI.detail).mockResolvedValue(pp({ rows: [row], total: 1 }));
  render(<App />);
  fireEvent.click(await screen.findByText('Empresa um'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  fireEvent.click(await screen.findByRole('cell', { name: 'CPE sintética' }));
  fireEvent.click(await screen.findByRole('tab', { name: 'Cliente' }));
  fireEvent.click(await screen.findByRole('link', { name: 'X' }));
  const session = await screen.findByRole('region', {
    name: 'Detalhes da sessão',
  });
  expect(within(session).getByRole('heading', { name: 'X' })).toBeVisible();
  expect(
    screen.getByRole('main').ownerDocument.defaultView?.location.hash,
  ).toBe('#sessao/X');
  expect(pppoeAPI.detail).toHaveBeenCalledWith(
    'company',
    'activation',
    'concentrator',
    'obs',
    'linked-row',
  );
  fireEvent.click(within(session).getByRole('button', { name: 'Voltar' }));
  expect(
    await screen.findByRole('heading', { name: 'CPE sintética' }),
  ).toBeVisible();
  expect(screen.getByRole('tab', { name: 'Cliente' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
});
