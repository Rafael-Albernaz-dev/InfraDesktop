import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
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
import { link as syntheticLink } from '../../features/radio/radio-test-fixtures';
import { PPPoE } from '../../features/pppoe/pppoe';
import { SessionRoute } from '../../features/pppoe/session-navigation';
import { useState } from 'react';

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
async function openList() {
  render(<App />);
  fireEvent.click(await screen.findByText('Empresa um'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  fireEvent.click(await screen.findByRole('cell', { name: 'CCR fixture' }));
  fireEvent.click(await screen.findByRole('tab', { name: /^Sessões$/ }));
  await screen.findByRole('button', { name: /Detalhar primeiro/ });
}

it.each(['button', 'alt-left', 'backspace', 'mouse'])(
  'returns from the session with %s and preserves the list page and search',
  async (trigger) => {
    await openList();
    fireEvent.change(screen.getByLabelText('Buscar sessões'), {
      target: { value: 'fixture' },
    });
    await waitFor(() =>
      expect(vi.mocked(pppoeAPI.list).mock.calls.at(-1)?.[4]?.search).toBe(
        'fixture',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Próxima página' }));
    fireEvent.click(
      await screen.findByRole('button', { name: /Detalhar segundo/ }),
    );
    const session = await screen.findByRole('region', {
      name: 'Detalhes da sessão',
    });
    expect(
      within(session).getByRole('heading', { name: 'segundo' }),
    ).toBeVisible();
    expect(
      screen.getByRole('main').ownerDocument.defaultView?.location.hash,
    ).toBe('#sessao/segundo');
    if (trigger === 'button')
      fireEvent.click(within(session).getByRole('button', { name: 'Voltar' }));
    if (trigger === 'alt-left')
      fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
        key: 'ArrowLeft',
        altKey: true,
      });
    if (trigger === 'backspace')
      fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
        key: 'Backspace',
      });
    if (trigger === 'mouse')
      fireEvent.mouseDown(screen.getByRole('main').ownerDocument, {
        button: 3,
      });
    expect(
      await screen.findByRole('button', { name: /Detalhar segundo/ }),
    ).toBeVisible();
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: /Detalhar segundo/ }),
      ).toHaveFocus(),
    );
    expect(screen.getByLabelText('Buscar sessões')).toHaveValue('fixture');
    expect(vi.mocked(pppoeAPI.list).mock.calls.at(-1)?.[5]).toBe('next');
  },
);

it('cancels a live observation on leaving and starts it again on return', async () => {
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    pp({ job: { id: 'live-fixture', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    pp({ job: { id: 'live-fixture', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledOnce());
  await waitFor(() => expect(pppoeReadAPI.status).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'activation',
      'live-fixture',
    ),
  );
  fireEvent.click(
    await screen.findByRole('button', { name: /Detalhar primeiro/ }),
  );
  await screen.findByRole('heading', { name: 'primeiro' });
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2));
});

it('cancels live on a tab change and starts it on returning to traffic', async () => {
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    pp({ job: { id: 'live-tab', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    pp({ job: { id: 'live-tab', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  await waitFor(() => expect(pppoeReadAPI.status).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('tab', { name: 'Informações' }));
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'activation',
      'live-tab',
    ),
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Tráfego' }));
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2));
  expect(
    screen.queryByRole('button', { name: 'Ver ao vivo' }),
  ).not.toBeInTheDocument();
});

it('shows an offline session without initiating traffic', async () => {
  vi.mocked(pppoeAPI.detail).mockResolvedValueOnce(
    pp({ rows: [{ ...rows[0], state: 'OFFLINE' }], total: 1 }),
  );
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  const session = await screen.findByRole('region', {
    name: 'Detalhes da sessão',
  });
  expect(
    within(session).getByText('Sem leitura: sessão OFFLINE'),
  ).toBeVisible();
  expect(
    within(session).queryByRole('button', { name: 'Ver ao vivo' }),
  ).not.toBeInTheDocument();
  expect(pppoeReadAPI.start).not.toHaveBeenCalled();
});

it('clears the visible session and its URL when changing company', async () => {
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    pp({ job: { id: 'live-scope', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    pp({ job: { id: 'live-scope', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  await waitFor(() => expect(pppoeReadAPI.status).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('link', { name: /Empresa um/ }));
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'activation',
      'live-scope',
    ),
  );
  expect(
    await screen.findByRole('heading', { name: 'Empresas' }),
  ).toBeVisible();
  fireEvent.click(screen.getByText('Empresa dois'));
  expect(
    await screen.findByRole('heading', { name: 'Visão geral' }),
  ).toBeVisible();
  expect(
    screen.queryByRole('region', { name: 'Detalhes da sessão' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('main').ownerDocument.defaultView?.location.hash,
  ).not.toContain('#sessao/');
});

it('shows the unbuilt history without a simulated table', async () => {
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  fireEvent.click(screen.getByRole('tab', { name: 'Histórico' }));
  const session = screen.getByRole('region', { name: 'Detalhes da sessão' });
  expect(
    within(session).getByText('Não feito · Ainda não existe (L-PULSO)'),
  ).toBeVisible();
  expect(within(session).queryByRole('table')).not.toBeInTheDocument();
});

it('keeps link review and manual fields available in the session screen', async () => {
  vi.mocked(radioAPI.link).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: { link: syntheticLink, total: 0, more: false, next_offset: 0 },
  });
  await openList();
  const cpe = {
    ...device,
    id: syntheticLink.cpe_id,
    role: RadioRoles.cpe,
    name: 'CPE fixture',
  };
  const ap = {
    ...device,
    id: syntheticLink.ap_id,
    role: RadioRoles.ap,
    name: 'AP fixture',
  };
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    ...equip,
    data: { ...equip.data, devices: [cpe, ap] },
  });
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  fireEvent.click(screen.getByRole('tab', { name: 'Enlace' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Revisar vínculo' }),
  );
  expect(await screen.findByLabelText('CPE')).toBeInTheDocument();
  expect(screen.getByLabelText('AP')).toBeInTheDocument();
  expect(screen.getByLabelText('MAC wireless da CPE')).toBeInTheDocument();
  expect(screen.getByLabelText('Interface do AP')).toBeInTheDocument();
  expect(screen.getByLabelText('BSSID do AP')).toBeInTheDocument();
  await screen.findByRole('option', { name: /CPE fixture/ });
  await screen.findByRole('option', { name: /AP fixture/ });
  fireEvent.change(screen.getByLabelText('CPE do vínculo'), {
    target: { value: syntheticLink.cpe_id },
  });
  fireEvent.change(screen.getByLabelText('AP do vínculo'), {
    target: { value: syntheticLink.ap_id },
  });
  expect(screen.getByLabelText('CPE do vínculo')).toHaveValue(
    syntheticLink.cpe_id,
  );
  expect(screen.getByLabelText('AP do vínculo')).toHaveValue(
    syntheticLink.ap_id,
  );
  fireEvent.change(screen.getByLabelText('MAC wireless da CPE'), {
    target: { value: '02:00:00:00:10:04' },
  });
  expect(screen.getByLabelText('MAC wireless da CPE')).toHaveValue(
    '02:00:00:00:10:04',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(
    screen.queryByRole('button', { name: 'Confirmar vínculo' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Atualizar' })).toBeEnabled();
});

it('pages the AP index for the exact session without a new remote read', async () => {
  const index = {
    observed_at: '2026-11-06T00:00:00Z',
    complete: true,
    targets: {},
    total: 12,
    offset: 0,
    next_offset: 8,
    more: true,
  };
  const reply = {
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: { index, total: 12, more: true, next_offset: 8 },
  };
  vi.mocked(radioAPI.link).mockResolvedValue(reply);
  vi.mocked(radioAPI.indexPage).mockResolvedValue({
    ...reply,
    data: { ...reply.data, index: { ...index, offset: 8, more: false } },
  });
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  fireEvent.click(screen.getByRole('tab', { name: 'Enlace' }));
  fireEvent.click(await screen.findByRole('button', { name: 'Próximos APs' }));
  await waitFor(() =>
    expect(radioAPI.indexPage).toHaveBeenCalledWith(
      'company',
      'activation',
      'ccr-fixture',
      'obs',
      'row-0',
      8,
    ),
  );
  expect(await screen.findByText('APs 9–12 de 12')).toBeVisible();
});

function ChangingTarget() {
  const [observation, setObservation] = useState('obs');
  return (
    <SessionRoute.Provider
      value={{
        active: true,
        target: {
          device: device.id,
          observation,
          row: 'row-0',
          login: 'primeiro',
        },
        open: () => {},
        back: () => {},
        leave: () => {},
      }}
    >
      <button onClick={() => setObservation('different-observation')}>
        Trocar alvo
      </button>
      <PPPoE
        profile="company"
        activation="activation"
        device={device}
        back={() => {}}
      />
    </SessionRoute.Provider>
  );
}

it('does not show a prior session when the target observation changes', async () => {
  vi.mocked(pppoeReadAPI.start).mockResolvedValue(
    pp({ job: { id: 'live-target', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(pppoeReadAPI.status).mockResolvedValue(
    pp({ job: { id: 'live-target', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  render(<ChangingTarget />);
  expect(
    await screen.findByRole('heading', { name: 'primeiro' }),
  ).toBeVisible();
  await waitFor(() => expect(pppoeReadAPI.status).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('button', { name: 'Trocar alvo' }));
  await waitFor(() =>
    expect(
      screen.queryByRole('heading', { name: 'primeiro' }),
    ).not.toBeInTheDocument(),
  );
  expect(await screen.findByText(/leitura da sessão mudou/i)).toBeVisible();
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'activation',
      'live-target',
    ),
  );
});

it('rejects a detail response whose login does not match the route target', async () => {
  vi.mocked(pppoeAPI.detail).mockResolvedValueOnce(
    pp({ rows: [{ ...rows[0], login: 'outro-usuario' }], total: 1 }),
  );
  render(<ChangingTarget />);
  expect(
    await screen.findByText(
      'A sessão retornada não corresponde ao alvo solicitado.',
    ),
  ).toBeVisible();
  expect(
    screen.queryByRole('heading', { name: 'outro-usuario' }),
  ).not.toBeInTheDocument();
});

it('runs ping only for the selected session from the information tab', async () => {
  vi.mocked(pppoeDiagnosticAPI.ping).mockResolvedValue(
    pp({ job: { id: 'ping-fixture', state: 'RUNNING', code: 'OK' }, total: 0 }),
  );
  vi.mocked(pppoeDiagnosticAPI.status).mockResolvedValue(
    pp({
      job: {
        id: 'ping-fixture',
        state: 'FAILED',
        code: 'OBSERVATION_INCOMPATIBLE',
      },
      total: 0,
    }),
  );
  await openList();
  fireEvent.click(screen.getByRole('button', { name: /Detalhar primeiro/ }));
  await screen.findByRole('heading', { name: 'primeiro' });
  fireEvent.click(screen.getByRole('tab', { name: 'Informações' }));
  const ping = await screen.findByRole('button', { name: 'Ping' });
  await waitFor(() => expect(ping).toBeEnabled());
  fireEvent.click(ping);
  await waitFor(() =>
    expect(pppoeDiagnosticAPI.ping).toHaveBeenCalledWith(
      'company',
      'activation',
      'ccr-fixture',
      'obs',
      'row-0',
      expect.any(String),
      1,
    ),
  );
});
