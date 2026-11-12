import { waitForSessionReady } from '../shared/testing/session-ready';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import App from './App';
import type {
  Device,
  DeviceInput,
  CredentialInput,
  Result,
} from '../platform/wails/presentation';
import { RadioTransports } from '../features/equipment/radio-contract';

function fixtureDaemon() {
  let snapshot: Result = {
    code: 'OK',
    request_id: 'synthetic',
    instance_id: 'synthetic',
    activation_id: 'a',
    profile_id: '',
    heartbeat: 1,
    authenticated: false,
    setup: true,
    name: '',
    theme: 'light',
    recovery: '',
    companies: [],
  };
  const devices: Device[] = [];
  const credentials = [
    {
      id: 'credential',
      alias: 'Leitura sintética',
      user: 'reader',
      revision: 1,
      links: 0,
    },
  ];
  const eq = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'a',
    data: {
      devices,
      credentials,
      total_devices: devices.length,
      vault_available: true,
      next_cursor: 0,
      ...data,
    },
  });
  const pp = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'a',
    data: { total: 0, ...data },
  });
  const row = {
    row_id: 'row',
    remote_id: '*1',
    session_id: 'session',
    login: 'cliente-sintetico',
    address: '192.0.2.101',
    caller_id: '02:00:5e:00:00:01',
    uptime: { min: 1e9, max: 1e9, known: true },
    state: 'ONLINE',
    duplicates: 1,
  };
  const current = {
    observation_id: 'observation',
    count: 1,
    online: 1,
    offline: 0,
    quality: { status: 'complete' },
    provenance: { ended_at: '2026-11-04T12:00:00Z' },
  };
  const reading = (scope: string) => ({
    scope,
    started_at: '2026-11-04T12:00:00Z',
    ended_at: '2026-11-04T12:00:02Z',
    duration_ns: 2e9,
    rows:
      scope === 'traffic'
        ? [
            {
              ...row,
              kind: 'traffic',
              traffic: {
                points: [
                  { t: 0, d: 1e6, u: 1e5 },
                  { t: 2000, d: 2e6, u: 2e5 },
                ],
                download_bytes: '10',
                upload_bytes: '2',
                covered_ns: 2e9,
                gaps: 0,
              },
            },
          ]
        : [],
  });
  const bindings = {
    Status: vi.fn(async () => snapshot),
    Login: vi.fn(async () => {
      snapshot = {
        ...snapshot,
        authenticated: true,
        setup: false,
        name: 'Operador',
        companies: [{ id: 'company', name: 'Empresa sintética' }],
      };
      return snapshot;
    }),
    SelectCompany: vi.fn(async () => {
      snapshot = { ...snapshot, profile_id: 'company' };
      return snapshot;
    }),
    GetBrand: vi.fn(async () => ({
      ...snapshot,
      brand: { name: '', logo: '', accent: 'teal' },
    })),
    ListEquipment: vi.fn(async () => eq()),
    ListCredentials: vi.fn(async () => eq()),
    GetEquipment: vi.fn(async () => eq({ device: devices[0] })),
    SaveEquipment: vi.fn(async (_p: string, _a: string, input: DeviceInput) => {
      devices.push({
        ...input,
        transport: RadioTransports.ssh,
        active: true,
        revision: 1,
        fingerprint: '',
        trust_origin: '',
      } as Device);
      return eq({ device: devices[0] });
    }),
    InspectTrust: vi.fn(async () =>
      eq({
        job: {
          id: 'trust',
          device_id: devices[0].id,
          kind: 'trust.inspect',
          state: 'SUCCEEDED',
          code: 'OK',
          fingerprint: 'SHA256:synthetic',
        },
      }),
    ),
    ConfirmTrust: vi.fn(async () => {
      devices[0] = {
        ...devices[0],
        fingerprint: 'SHA256:synthetic',
        revision: 2,
      };
      return eq({ device: devices[0] });
    }),
    StartQuery: vi.fn(async () =>
      eq({
        job: {
          id: 'query',
          device_id: devices[0].id,
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      }),
    ),
    PutCredential: vi.fn(
      async (_p: string, _a: string, input: CredentialInput) => {
        credentials.push({
          id: input.id,
          alias: input.alias,
          user: input.user,
          revision: 1,
          links: 0,
        });
        return eq();
      },
    ),
    GetPPPoECurrent: vi.fn(async () => pp({ current })),
    ListPPPoEReferences: vi.fn(async () => pp({ references: [] })),
    ListPPPoESessions: vi.fn(async () => pp({ rows: [row], total: 1 })),
    GetPPPoESession: vi.fn(async () => pp({ rows: [row], total: 1 })),
    PPPoEContextCounts: vi.fn(async () => pp({ groups: [] })),
    GetDeviceMonitoring: vi.fn(async () => pp({ read: reading('equipment') })),
    GetPPPoEDiagnostic: vi.fn(async () => pp()),
    GetRadioLink: vi.fn(async () => pp()),
    GetRadioIndexPage: vi.fn(async () => pp()),
    StartPPPoERead: vi.fn(
      async (_p: string, _a: string, _d: string, _op: string, scope: string) =>
        pp({ job: { id: scope, state: 'RUNNING', code: 'OK' } }),
    ),
    PPPoEReadStatus: vi.fn(async (_p: string, _a: string, scope: string) =>
      pp({
        job: {
          id: scope,
          state: scope === 'traffic' ? 'RUNNING' : 'SUCCEEDED',
          code: 'OK',
        },
        read: reading(scope),
      }),
    ),
    StopPPPoETraffic: vi.fn(async () =>
      pp({
        job: { id: 'traffic', state: 'CANCELLED', code: 'CANCELLED' },
        read: reading('traffic'),
      }),
    ),
    CancelPPPoEReadOperation: vi.fn(async () => pp()),
    CancelPPPoERead: vi.fn(async () => pp()),
  };
  return { bindings, devices };
}
afterEach(() => vi.stubGlobal('go', undefined));

it('mantém o percurso de conta, empresa, cadastro, credencial, ficha e sessão ligado aos bindings existentes', async () => {
  const { bindings, devices } = fixtureDaemon();
  vi.stubGlobal('go', { presentation: { App: bindings } });
  render(<App />);
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeEnabled(),
  );
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'operador' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'senha exclusivamente sintética' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
  fireEvent.click(
    await screen.findByRole('cell', { name: 'Empresa sintética' }),
  );
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar equipamento' }),
  );
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  const form = within(screen.getByRole('main'));
  fireEvent.change(form.getByLabelText('Nome'), {
    target: { value: 'Concentrador sintético' },
  });
  fireEvent.change(form.getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.1' },
  });
  fireEvent.change(form.getByLabelText('Papel do equipamento'), {
    target: { value: 'concentrator' },
  });
  if (form.queryByRole('button', { name: 'Usar credencial existente' }))
    fireEvent.click(
      form.getByRole('button', { name: 'Usar credencial existente' }),
    );
  fireEvent.change(form.getByLabelText('Credencial de consulta'), {
    target: { value: 'credential' },
  });
  fireEvent.click(form.getByRole('button', { name: /^Salvar$/ }));
  fireEvent.click(
    await screen.findByRole('cell', { name: 'Concentrador sintético' }),
  );
  expect(bindings.InspectTrust).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole('button', { name: 'Conectar' }));
  const trust = within(
    await screen.findByRole('dialog', { name: 'Primeira conexão' }),
  );
  expect(bindings.StartQuery).not.toHaveBeenCalled();
  fireEvent.click(
    trust.getByRole('button', { name: 'Confiar neste equipamento e conectar' }),
  );
  await waitFor(() => expect(bindings.StartQuery).toHaveBeenCalledOnce());
  expect(bindings.Login).toHaveBeenCalledWith(
    'operador',
    'senha exclusivamente sintética',
    true,
  );
  expect(bindings.SelectCompany).toHaveBeenCalledWith('company');
  expect(bindings.SaveEquipment).toHaveBeenCalledOnce();
  expect(bindings.ConfirmTrust).toHaveBeenCalledWith(
    'company',
    'a',
    devices[0].id,
    expect.any(String),
    'trust',
    'SHA256:synthetic',
    1,
  );
  fireEvent.click(screen.getByRole('link', { name: 'Credenciais' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar credencial' }),
  );
  const credential = within(
    await screen.findByRole('dialog', { name: 'Credenciais de consulta' }),
  );
  fireEvent.change(credential.getByLabelText('Alias'), {
    target: { value: 'Nova leitura' },
  });
  fireEvent.change(credential.getByLabelText('Usuário remoto'), {
    target: { value: 'novo-reader' },
  });
  fireEvent.change(credential.getByLabelText('Senha remota'), {
    target: { value: 'credencial exclusivamente sintética' },
  });
  fireEvent.click(
    credential.getByRole('button', { name: 'Salvar credencial' }),
  );
  await waitFor(() => expect(bindings.PutCredential).toHaveBeenCalledOnce());
  fireEvent.click(screen.getByRole('link', { name: 'Lista' }));
  fireEvent.click(
    await screen.findByRole('cell', { name: 'Concentrador sintético' }),
  );
  for (const tab of [
    'Resumo',
    'Interfaces e saúde',
    'Serviços',
    'Acesso',
    'Sessões',
  ]) {
    fireEvent.click(await screen.findByRole('tab', { name: tab }));
    expect(screen.getByRole('tab', { name: tab })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  }
  fireEvent.click(
    await screen.findByRole('button', {
      name: 'Detalhar cliente-sintetico *1',
    }),
  );
  const session = within(await waitForSessionReady());
  await waitFor(() =>
    expect(bindings.StartPPPoERead).toHaveBeenCalledWith(
      'company',
      'a',
      devices[0].id,
      expect.any(String),
      'traffic',
      'observation',
      ['row'],
      2,
    ),
  );
  fireEvent.click(await session.findByRole('button', { name: 'Parar' }));
  await waitFor(() => expect(bindings.StopPPPoETraffic).toHaveBeenCalledOnce());
  fireEvent.click(await session.findByRole('button', { name: 'Ver ao vivo' }));
  await waitFor(() =>
    expect(
      bindings.StartPPPoERead.mock.calls.filter(
        (call) => call[4] === 'traffic',
      ),
    ).toHaveLength(2),
  );
  fireEvent.click(session.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(bindings.StopPPPoETraffic).toHaveBeenCalledTimes(2),
  );
  fireEvent.click(session.getByRole('button', { name: 'Medir banda' }));
  await waitFor(() =>
    expect(
      bindings.StartPPPoERead.mock.calls.some(
        (call) => call[4] === 'bandwidth',
      ),
    ).toBe(true),
  );
  fireEvent.click(session.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  expect(
    await screen.findByRole('heading', { name: 'Equipamentos' }),
  ).toBeVisible();
});
