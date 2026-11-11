import { device, data } from '../../shared/testing/equipment-test-fixture';

import { beginAdd } from '../test-navigation';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';

import { equipmentAPI as api } from '../../features/equipment/equipment-api';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({
    deviceMonitoringAPI: {
      current: vi.fn().mockResolvedValue({
        code: 'OK',
        profile_id: 'company',
        activation_id: 'active',
        data: { total: 0 },
      }),
    },
  }),
);
beforeEach(resetEquipmentMocks);
import { resetEquipmentMocks } from '../../shared/testing/equipment-setup';
it('first enrollment retains one CTA and the frozen empty inventory layout', async () => {
  vi.mocked(api.list).mockResolvedValue(data([]));
  vi.mocked(api.credentials).mockResolvedValue(data([]));
  render(<Equipment profile="X" activation="a" connected />);
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(
    screen.getByRole('table', { name: 'Lista de equipamentos' }),
  ).toBeVisible();
  expect(screen.queryByLabelText('Buscar equipamento')).not.toBeInTheDocument();
  expect(
    screen.queryByLabelText('Estado dos equipamentos'),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByText(/equipamentos nesta página/),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByText(/Selecione um equipamento para ver/),
  ).not.toBeInTheDocument();
  const add = screen.getAllByRole('button', { name: /Adicionar/ });
  expect(add).toHaveLength(1);
  fireEvent.click(add[0]);
  expect(
    screen.getByRole('heading', { name: 'Adicionar equipamento' }),
  ).toBeVisible();
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__add__' },
  });
  const dialog = screen.getByRole('dialog', {
    name: 'Credenciais de consulta',
  });
  expect(within(dialog).getByLabelText('Usuário remoto')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('Alias')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('Senha remota')).toHaveAttribute(
    'type',
    'password',
  );
});
it('lost direct response clears the password, blocks blind resend and reconciles after closing/reopening', async () => {
  let submitted: Record<string, unknown> | undefined;
  vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
    submitted = structuredClone(d);
    return { ...data(), code: 'SERVICE_UNAVAILABLE' };
  });
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...data(),
    data: { ...data().data!, operation_state: 'COMMITTED', device },
  });
  render(<Equipment profile="direct-lost" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  for (const [label, value] of [
    ['Nome', 'New direct'],
    ['IP de gerenciamento', '127.0.0.1'],
    ['Usuário', 'reader'],
    ['Senha', 'transient fixture secret'],
  ]) {
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  }
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(submitted).toMatchObject({
    user: 'reader',
    secret: 'transient fixture secret',
    credential_id: '',
  });
  expect(
    screen.queryByDisplayValue('transient fixture secret'),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /^Salvar$/ }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  await beginAdd();
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await waitFor(() =>
    expect(api.enrollmentStatus).toHaveBeenCalledWith(
      'direct-lost',
      'a',
      submitted?.id,
      submitted?.operation_id,
    ),
  );
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(api.credentials).toHaveBeenCalledTimes(2);
  expect(JSON.stringify(localStorage)).not.toContain(
    'transient fixture secret',
  );
});
it('a missing receipt permits only a deliberate retry with the original IDs, never a fresh duplicate', async () => {
  const sent: Record<string, unknown>[] = [];
  vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
    sent.push(structuredClone(d));
    return { ...data(), code: 'DEADLINE_EXCEEDED' };
  });
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...data(),
    data: { ...data().data!, operation_state: 'NOT_FOUND' },
  });
  render(<Equipment profile="direct-retry" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  const fill = () => {
    for (const [label, value] of [
      ['Nome', 'Retry'],
      ['IP de gerenciamento', '127.0.0.1'],
      ['Usuário', 'reader'],
      ['Senha', 'retry fixture secret'],
    ])
      fireEvent.change(screen.getByLabelText(label), { target: { value } });
  };
  fill();
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  fill();
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await waitFor(() => expect(sent).toHaveLength(2));
  expect(sent[1].id).toBe(sent[0].id);
  expect(sent[1].operation_id).toBe(sent[0].operation_id);
});
it('creation with an existing credential also reconciles a lost response without changing identity', async () => {
  let sent: Record<string, unknown> | undefined;
  vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
    sent = structuredClone(d);
    return { ...data(), code: 'DEADLINE_EXCEEDED' };
  });
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...data(),
    data: { ...data().data!, operation_state: 'COMMITTED', device },
  });
  render(<Equipment profile="reuse-lost" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'Reuse' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '127.0.0.1' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: 'credential' },
  });
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  expect(sent).toMatchObject({
    credential_id: 'credential',
    user: '',
    secret: '',
  });
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  await beginAdd();
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await waitFor(() =>
    expect(api.enrollmentStatus).toHaveBeenCalledWith(
      'reuse-lost',
      'a',
      sent?.id,
      sent?.operation_id,
    ),
  );
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(api.save).toHaveBeenCalledTimes(1);
});
it('direct creation exposes user and password without an earlier credential form', async () => {
  render(<Equipment profile="fields-red" activation="a" connected addPage />);
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  expect(screen.getByLabelText('Usuário')).toBeInTheDocument();
  expect(screen.getByLabelText('Senha')).toHaveAttribute('type', 'password');
  expect(screen.queryByLabelText('Alias')).not.toBeInTheDocument();
});
