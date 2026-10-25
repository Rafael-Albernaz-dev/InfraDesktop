import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { equipmentAPI as api } from './equipment-api';
import type { EquipmentResult } from './equipment-api';
vi.mock('./presentation', () => ({
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
async function open(profile: string) {
  render(
    <Equipment profile={profile} activation="a" connected createRequest={0} />,
  );
  const opener = await screen.findByRole('button', {
    name: 'Adicionar equipamento',
  });
  opener.focus();
  fireEvent.click(opener);
}
function fill(address = '10.0.0.1') {
  for (const [label, value] of [
    ['Nome', 'Router teste'],
    ['IP de gerenciamento', address],
    ['Usuário', 'reader'],
    ['Senha', '  synthetic secret  '],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
}
const save = () =>
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
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
it('keeps the draft when the equipment backdrop is clicked', async () => {
  await open('backdrop');
  fill();
  fireEvent.click(screen.getByRole('dialog').parentElement!);
  expect(screen.getByRole('dialog')).toBeInTheDocument();
  expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
});
it.each(['Escape', 'X', 'Cancelar'])(
  'routes dirty %s through discard confirmation and preserves all fields on cancel',
  async (kind) => {
    await open('dirty-' + kind);
    fill();
    const ip = screen.getByLabelText('IP de gerenciamento');
    ip.focus();
    if (kind === 'Escape') fireEvent.keyDown(ip, { key: 'Escape' });
    else
      fireEvent.click(
        screen.getByRole('button', {
          name: kind === 'X' ? 'Fechar painel' : 'Cancelar',
        }),
      );
    const confirm = screen.getByRole('dialog', {
      name: 'Descartar alterações?',
    });
    fireEvent.click(confirm.parentElement!);
    expect(confirm).toBeInTheDocument();
    fireEvent.click(
      within(confirm).getByRole('button', { name: 'Continuar editando' }),
    );
    expect(screen.getByLabelText('Nome')).toHaveValue('Router teste');
    expect(ip).toHaveValue('10.0.0.1');
    expect(screen.getByLabelText('Usuário')).toHaveValue('reader');
    expect(screen.getByLabelText('Porta SSH')).toHaveValue(22);
    expect(screen.getByLabelText('Senha')).toHaveValue('  synthetic secret  ');
    expect(ip).toHaveFocus();
    fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
    fireEvent.click(
      within(
        screen.getByRole('dialog', { name: 'Descartar alterações?' }),
      ).getByRole('button', { name: 'Descartar alterações' }),
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(api.save).not.toHaveBeenCalled();
  },
);
it('blocks close during send and retains uncertain receipt IDs without another password send', async () => {
  let resolve!: (r: EquipmentResult) => void;
  let sent: Record<string, unknown> = {};
  vi.mocked(api.save).mockImplementation((_p, _a, d) => {
    sent = { ...d };
    return new Promise((r) => {
      resolve = r;
    });
  });
  await open('pending-guard');
  fill();
  save();
  const dialog = screen.getByRole('dialog');
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.keyDown(dialog, { key: 'Escape' });
  fireEvent.click(dialog.parentElement!);
  expect(
    screen.getByRole('dialog', { name: 'Adicionar equipamento' }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('dialog', { name: 'Descartar alterações?' }),
  ).not.toBeInTheDocument();
  await act(async () => resolve(result('SERVICE_UNAVAILABLE')));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  const confirmation = screen.getByRole('dialog', {
    name: 'Descartar alterações?',
  });
  fireEvent.click(
    within(confirmation).getByRole('button', { name: 'Descartar alterações' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
  expect(
    screen.getByRole('button', { name: 'Conferir cadastro' }),
  ).toBeInTheDocument();
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...result(),
    data: { ...result().data!, operation_state: 'NOT_FOUND' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  fill();
  save();
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.save).mock.calls[1][2].id).toBe(sent.id);
  expect(vi.mocked(api.save).mock.calls[1][2].operation_id).toBe(
    sent.operation_id,
  );
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
it('protects credential drafts against backdrop and dirty close', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  render(
    <Equipment
      profile="credential-dirty"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Credenciais' }));
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic credential' },
  });
  const dialog = screen.getByRole('dialog');
  fireEvent.click(dialog.parentElement!);
  expect(dialog).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Continuar editando' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue(
    'synthetic credential',
  );
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
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );
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
it('closes a pristine draft, restores opener and keeps Escape/X on confirmation safe', async () => {
  await open('focus-clean');
  const opener = screen.getByRole('button', { name: 'Adicionar equipamento' });
  fireEvent.keyDown(screen.getByLabelText('Nome'), { key: 'Escape' });
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(opener).toHaveFocus();
  fireEvent.click(opener);
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
it('blocks all credential closing routes during a send and clears the request object', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  let resolve!: (r: EquipmentResult) => void;
  let request: { secret: string } | undefined;
  vi.mocked(api.putCredential).mockImplementation((_p, _a, d) => {
    request = d;
    return new Promise((r) => {
      resolve = r;
    });
  });
  render(
    <Equipment
      profile="credential-send"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Credenciais' }));
  for (const [label, value] of [
    ['Alias', 'Consulta'],
    ['Usuário remoto', 'reader'],
    ['Senha remota', 'synthetic'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  const dialog = screen.getByRole('dialog');
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.keyDown(dialog, { key: 'Escape' });
  fireEvent.click(dialog.parentElement!);
  expect(dialog).toBeInTheDocument();
  expect(
    screen.queryByRole('dialog', { name: 'Descartar alterações?' }),
  ).not.toBeInTheDocument();
  await act(async () => resolve(result('INVALID_REQUEST')));
  expect(request?.secret).toBe('');
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(screen.getByRole('alert')).toHaveTextContent(/O serviço recusou/);
});

it('requires discard before switching a dirty credential and cancel preserves the draft', async () => {
  const credential = {
    id: 'shared',
    alias: 'Shared',
    user: 'reader',
    links: 2,
    revision: 1,
  };
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  vi.mocked(api.credentials).mockResolvedValue({
    ...result(),
    data: { ...result().data!, credentials: [credential] },
  });
  render(
    <Equipment
      profile="credential-switch"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Credenciais' }));
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic' },
  });
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'shared' },
  });
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Continuar editando' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue('synthetic');
  expect(screen.getByLabelText('Credencial para editar')).toHaveValue('');
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'shared' },
  });
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(screen.getByLabelText('Alias')).toHaveValue('Shared');
  expect(
    screen.getByRole('button', { name: 'Salvar credencial' }),
  ).toBeDisabled();
  expect(api.putCredential).not.toHaveBeenCalled();
});
