import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Equipment } from '../pages/equipment';
import { RadioEquipment } from '../../features/radio/radio';
import { AccessScreen } from '../../features/access/access-screen';
import {
  device as radioDevice,
  reading,
  response,
} from '../../features/radio/radio-test-fixtures';
import { RadioCodes } from '../../features/equipment/radio-contract';
import type { Device } from '../../features/equipment/equipment-api';

function localBindings(change: Partial<Device> = {}, vault = true) {
  const device = {
    ...radioDevice,
    id: 'router',
    name: 'Router sintético',
    role: 'concentrator',
    credential_id: 'credential',
    address: '192.0.2.1',
    port: 22,
    ...change,
  };
  const eq = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [device],
      credentials: [],
      next_cursor: 0,
      vault_available: vault,
      ...data,
    },
  });
  const pp = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: { total: 0, ...data },
  });
  return {
    ListEquipment: vi.fn(async () => eq()),
    GetEquipment: vi.fn(async () => eq({ device })),
    ListCredentials: vi.fn(async () => eq()),
    GetPPPoECurrent: vi.fn(async () => pp()),
    ListPPPoEReferences: vi.fn(async () => pp({ references: [] })),
    ListPPPoESessions: vi.fn(async () => pp({ rows: [] })),
    GetPPPoEDiagnostic: vi.fn(async () => pp()),
    StartPPPoECollection: vi.fn(async () =>
      pp({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
    PPPoECollectionStatus: vi.fn(async () =>
      pp({ job: { id: 'collect', state: 'RUNNING', code: 'OK' } }),
    ),
    CancelPPPoECollection: vi.fn(async () =>
      pp({ job: { id: 'collect', state: 'CANCELLED', code: 'CANCELLED' } }),
    ),
    StartQuery: vi.fn(async () => eq()),
    InspectTrust: vi.fn(async () => eq()),
    GetRadio: vi.fn(async () => response({ reading })),
    StartRadio: vi.fn(async () => ({
      ...response(),
      code: RadioCodes.REMOTE_AUTH_FAILED,
    })),
    CancelRadioOperation: vi.fn(async () => response()),
    CopyRecovery: vi.fn(async () => 'UNAVAILABLE'),
    SaveRecovery: vi.fn(async () => 'UNAVAILABLE'),
  };
}
function install(bindings: ReturnType<typeof localBindings>) {
  vi.stubGlobal('go', { presentation: { App: bindings } });
}
afterEach(() => vi.stubGlobal('go', undefined));
async function showSheet(bindings = localBindings()) {
  install(bindings);
  render(
    <Equipment
      profile="company"
      activation="active"
      connected

      targetID="router"
    />,
  );
  await screen.findByRole('heading', { name: 'Router sintético' });
  return bindings;
}
it('troca diretamente Sessões → Serviços → Sessões com conteúdo e subaba da seção atual', async () => {
  await showSheet();
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  expect(await screen.findByRole('tab', { name: 'Lista' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  fireEvent.click(screen.getByRole('tab', { name: 'Serviços' }));
  expect(
    await screen.findByRole('tab', { name: 'VLANs e serviços' }),
  ).toHaveAttribute('aria-selected', 'true');
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  expect(await screen.findByRole('tab', { name: 'Lista' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
});
it('mantém Parar disponível em Referências e cancela o mesmo job sem nova coleta', async () => {
  const bindings = await showSheet();
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Atualizar' })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  await screen.findByRole('button', { name: 'Parar' });
  fireEvent.click(screen.getByRole('tab', { name: 'Referências' }));
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(bindings.CancelPPPoECollection).toHaveBeenCalledWith(
      'company',
      'active',
      'collect',
    ),
  );
  expect(bindings.StartPPPoECollection).toHaveBeenCalledOnce();
});
it('mantém o motivo de falha de rádio junto da leitura anterior', async () => {
  install(localBindings());
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={radioDevice}
      blocked={false}
      tab="Resumo"
    />,
  );
  await screen.findByText('Modelo');
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(/recus/i);
  expect(screen.getByText('Modelo')).toBeVisible();
  expect(
    screen.getByText('Identidade e sistema').closest('[data-state]'),
  ).toHaveAttribute('data-state', 'failed');
});
it('conserva a seleção manual por teclado quando copiar o código falha', async () => {
  install(localBindings());
  render(
    <AccessScreen
      setup
      connected
      busy={false}
      error=""
      recovery="XXXX-XXXX-XXXX"
      acknowledge={() => {}}
      submit={() => {}}
    />,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Copiar código' }));
  await screen.findByRole('status');
  const code = screen.getByRole('textbox', {
    name: 'Código de recuperação',
  }) as HTMLTextAreaElement;
  fireEvent.focus(code);
  expect(code).toHaveValue('XXXX-XXXX-XXXX');
  expect(code.readOnly).toBe(true);
  expect([code.selectionStart, code.selectionEnd]).toEqual([0, 14]);
});
for (const [reason, change, vault] of [
  ['inativo', { active: false }, true],
  ['sem credencial', { credential_id: '' }, true],
  ['sem cofre', {}, false],
] as const) {
  it(`mantém a mesma recusa de início no cabeçalho e no bloco ${reason}`, async () => {
    const bindings = await showSheet(localBindings(change, vault));
    const block = screen.getByText('Identidade e sistema').closest('.block')!;
    expect(
      within(block as HTMLElement).getByRole('button', { name: 'Atualizar' }),
    ).toBeDisabled();
    expect(bindings.StartQuery).not.toHaveBeenCalled();
  });
}
it('permite a leitura explícita pelo bloco quando cofre, atividade e credencial estão presentes', async () => {
  const bindings = await showSheet();
  const block = screen.getByText('Identidade e sistema').closest('.block')!;
  fireEvent.click(
    within(block as HTMLElement).getByRole('button', { name: 'Atualizar' }),
  );
  await waitFor(() => expect(bindings.StartQuery).toHaveBeenCalledOnce());
});

it('não abre confiança nem aplica resultado tardio da ficha depois de trocar para Credenciais', async () => {
  const bindings = localBindings({ fingerprint: '' });
  let complete!: () => void;
  bindings.InspectTrust.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        complete = () =>
          resolve({
            code: 'OK',
            profile_id: 'company',
            activation_id: 'active',
            data: {
              devices: [],
              credentials: [],
              next_cursor: 0,
              vault_available: true,
              job: {
                id: 'trust',
                device_id: 'router',
                kind: 'trust.inspect',
                state: 'SUCCEEDED',
                code: 'OK',
                fingerprint: 'SHA256:synthetic',
              },
            },
          } as Awaited<ReturnType<typeof bindings.InspectTrust>>);
      }),
  );
  install(bindings);
  const view = render(
    <Equipment
      profile="company"
      activation="active"
      connected

      targetID="router"
    />,
  );
  await screen.findByRole('heading', { name: 'Router sintético' });
  fireEvent.click(screen.getByRole('button', { name: 'Conectar' }));
  await waitFor(() => expect(bindings.InspectTrust).toHaveBeenCalledOnce());
  view.rerender(
    <Equipment
      credentialsPage
      profile="company"
      activation="active"
      connected

      targetID="router"
    />,
  );
  await screen.findByRole('heading', { name: 'Credenciais' });
  complete();
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(bindings.StartQuery).not.toHaveBeenCalled();
});
