import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../pages/equipment';
import { Equipment as TestEquipment } from '../test-equipment-shell';
import { RadioTransports } from '../../features/equipment/radio-contract';
import { equipmentAPI as api } from '../../features/equipment/equipment-api';
import type { EquipmentResult } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', () => ({
  equipmentAPI: Object.fromEntries(
    [
      'list',
      'credentials',
      'save',
      'enrollmentStatus',
      'putCredential',
      'get',
      'active',
      'inspect',
      'confirm',
      'start',
      'status',
      'cancel',
    ].map((k) => [k, vi.fn()]),
  ),
}));
const result = (code = 'OK'): EquipmentResult => ({
  code,
  profile_id: 'modal',
  activation_id: 'a',
  data: { devices: [], credentials: [], next_cursor: 0, vault_available: true },
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.list).mockResolvedValue(result());
  vi.mocked(api.credentials).mockResolvedValue(result());
});
const back = vi.fn();
async function open(profile: string) {
  const view = render(
    <Equipment
      profile={profile}
      activation="a"
      connected
      addPage
      onAddBack={back}
    />,
  );
  await screen.findByRole('button', { name: 'Salvar' });
  return view;
}
function fill(address = '10.0.0.1') {
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  for (const [label, value] of [
    ['Nome', 'Router teste'],
    ['IP de gerenciamento', address],
    ['Usuário', 'reader'],
    ['Senha', '  synthetic secret  '],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
}
const save = () =>
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
it('localizes a dotless IP and focuses it without API or remote work', async () => {
  await open('invalid-local');
  fill('10001');
  save();
  const ip = screen.getByLabelText('IP de gerenciamento');
  expect(ip).toHaveAttribute('aria-invalid', 'true');
  expect(ip).toHaveFocus();
  expect(ip).toHaveAccessibleDescription(/IPv4.*10\.0\.0\.1.*IPv6/);
  expect(api.save).not.toHaveBeenCalled();
  expect(api.start).not.toHaveBeenCalled();
  expect(api.inspect).not.toHaveBeenCalled();
  expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
});
it('associates safe daemon validation with the field and clears only the submitted password', async () => {
  vi.mocked(api.save).mockResolvedValue(result('DEVICE_ADDRESS_INVALID'));
  await open('daemon-field');
  fill();
  save();
  await waitFor(() =>
    expect(screen.getByLabelText('IP de gerenciamento')).toHaveAttribute(
      'aria-invalid',
      'true',
    ),
  );
  expect(screen.getByLabelText('IP de gerenciamento')).toHaveFocus();
  expect(screen.getByLabelText('Nome')).toHaveValue('Router teste');
  expect(screen.getByLabelText('Usuário')).toHaveValue('reader');
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  expect(screen.getByText(/Digite a senha novamente/)).toBeInTheDocument();
});
it.each([
  '10.0.0.1',
  ' 10.0.0.1 ',
  '2001:DB8:0:0:0:0:0:1',
  '::1',
  '2001:db8::1',
  '::ffff:192.0.2.1',
  '2001:db8:0:1:2:3:192.0.2.1',
])(
  'accepts an unambiguous literal %s while retaining exact password bytes',
  async (address) => {
    let sent: Record<string, unknown> = {};
    vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
      sent = { ...d };
      return result();
    });
    await open('valid-' + address);
    fill(address);
    save();
    await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
    expect(sent.secret).toBe('  synthetic secret  ');
    expect(sent.address).toBe(address.trim());
    expect(api.start).not.toHaveBeenCalled();
    expect(api.inspect).not.toHaveBeenCalled();
    await waitFor(() => expect(back).toHaveBeenCalled());
  },
);
it.each([
  '10.1',
  '010.0.0.1',
  '256.0.0.1',
  '0.0.0.0',
  '224.0.0.1',
  '::',
  'ff02::1',
  '2001:::1',
  '2001:db8::1::2',
  '2001:db8:1',
  '1:2:3:4:5:6:7:8:9',
  '1:2:3:4:5:6:7::8',
  'fe80::1%eth0',
  '[2001:db8::1]',
  '10.0.0.1:22',
  'reader@10.0.0.1',
  'ssh://10.0.0.1',
])(
  'rejects ambiguous or non-unicast address %s without submission',
  async (address) => {
    await open('reject-' + address);
    fill(address);
    save();
    expect(screen.getByLabelText('IP de gerenciamento')).toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(api.save).not.toHaveBeenCalled();
  },
);
it.each([
  ['Nome', 'bad\u0001name'],
  ['Usuário', 'bad\u0001user'],
  ['Porta SSH', '0'],
  ['Porta SSH', '22.5'],
  ['Senha', ''],
])(
  'localizes invalid %s and preserves non-secret fields',
  async (label, value) => {
    await open('field-' + label + value);
    fill();
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
    save();
    expect(screen.getByLabelText(label)).toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(screen.getByLabelText(label)).toHaveFocus();
    expect(api.save).not.toHaveBeenCalled();
  },
);

it.each(['Voltar', 'Cancelar'])(
  'protects a dirty draft when leaving by %s',
  async (button) => {
    back.mockClear();
    await open('dirty-page-' + button);
    fill();
    fireEvent.click(screen.getByRole('button', { name: button }));
    const confirm = screen.getByRole('dialog', {
      name: 'Descartar alterações?',
    });
    fireEvent.click(confirm.parentElement!);
    expect(confirm).toBeInTheDocument();
    fireEvent.click(
      within(confirm).getByRole('button', { name: 'Continuar editando' }),
    );
    expect(screen.getByLabelText('Nome')).toHaveValue('Router teste');
    expect(screen.getByLabelText('IP de gerenciamento')).toHaveValue(
      '10.0.0.1',
    );
    expect(screen.getByLabelText('Usuário')).toHaveValue('reader');
    expect(screen.getByLabelText('Porta SSH')).toHaveValue(22);
    expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
    fireEvent.click(screen.getByRole('button', { name: button }));
    fireEvent.click(
      within(
        screen.getByRole('dialog', { name: 'Descartar alterações?' }),
      ).getByRole('button', { name: 'Descartar alterações' }),
    );
    expect(back).toHaveBeenCalledOnce();
    expect(api.save).not.toHaveBeenCalled();
  },
);

it('blocks leaving during send and reconciles an uncertain receipt on reopening', async () => {
  back.mockClear();
  let resolve!: (r: EquipmentResult) => void;
  let sent: Record<string, unknown> = {};
  vi.mocked(api.save).mockImplementation((_p, _a, d) => {
    sent = { ...d };
    return new Promise((r) => {
      resolve = r;
    });
  });
  const view = await open('pending-page');
  fill();
  save();
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(back).not.toHaveBeenCalled();
  expect(
    screen.queryByRole('dialog', { name: 'Descartar alterações?' }),
  ).not.toBeInTheDocument();
  await act(async () => resolve(result('SERVICE_UNAVAILABLE')));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  expect(back).toHaveBeenCalledOnce();
  view.unmount();
  render(
    <Equipment
      profile="pending-page"
      activation="a"
      connected
      addPage
      onAddBack={back}
    />,
  );
  expect(
    await screen.findByRole('button', { name: 'Conferir cadastro' }),
  ).toBeInTheDocument();
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...result(),
    data: { ...result().data!, operation_state: 'NOT_FOUND' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  fill();
  save();
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.save).mock.calls[1][2].id).toBe(sent.id);
  expect(vi.mocked(api.save).mock.calls[1][2].operation_id).toBe(
    sent.operation_id,
  );
});

it('returns directly for a pristine draft and keeps confirmation dismissal safe', async () => {
  back.mockClear();
  await open('focus-page');
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(back).toHaveBeenCalledOnce();
  fill();
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  let confirmation = screen.getByRole('dialog', {
    name: 'Descartar alterações?',
  });
  expect(
    within(confirmation).getByRole('button', { name: 'Fechar painel' }),
  ).toHaveFocus();
  fireEvent.keyDown(confirmation, { key: 'Escape' });
  expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  confirmation = screen.getByRole('dialog', { name: 'Descartar alterações?' });
  fireEvent.click(
    within(confirmation).getByRole('button', { name: 'Fechar painel' }),
  );
  expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
});

it('saving a device with a returned record never starts a remote inspection', async () => {
  const device = {
    id: 'fixture-device',
    name: 'Router teste',
    role: 'other',
    address: '192.0.2.1',
    port: 22,
    revision: 1,
    active: true,
    fingerprint: '',
    block: '',
    transport: RadioTransports.ssh,
    credential_id: '',
    trust_origin: '',
  };
  vi.mocked(api.save).mockResolvedValue({
    ...result(),
    data: { ...result().data!, device },
  });
  vi.mocked(api.get).mockResolvedValue({
    ...result(),
    profile_id: 'no-query-after-save',
    data: { ...result().data!, device },
  });
  vi.mocked(api.inspect).mockResolvedValue({
    ...result(),
    profile_id: 'no-query-after-save',
  });
  await open('no-query-after-save');
  fill('192.0.2.1');
  save();
  await waitFor(() => expect(api.save).toHaveBeenCalledOnce());
  await waitFor(() => expect(back).toHaveBeenCalledOnce());
  expect(api.inspect).not.toHaveBeenCalled();
  expect(api.start).not.toHaveBeenCalled();
});

it('returns focus to the Add trigger after cancelling a pristine page', async () => {
  render(<TestEquipment profile="focus-origin" activation="a" connected />);
  const trigger = await screen.findByRole('button', {
    name: 'Adicionar equipamento',
  });
  trigger.focus();
  fireEvent.click(trigger);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(
    await screen.findByRole('button', { name: 'Adicionar equipamento' }),
  ).toHaveFocus();
});
