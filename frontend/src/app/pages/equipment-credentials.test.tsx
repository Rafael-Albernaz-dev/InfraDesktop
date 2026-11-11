import { data } from '../../shared/testing/equipment-test-fixture';

import { beginAdd } from '../test-navigation';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';

import { equipmentAPI as api } from '../../features/equipment/equipment-api';

import type { EquipmentResult } from '../../features/equipment/equipment-api';
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
it('shared credential replacement sends affected links and clears typed secret', async () => {
  vi.mocked(api.putCredential).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  render(<Equipment credentialsPage profile="X" activation="a" connected />);
  await screen.findByRole('heading', { name: 'Credenciais' });
  fireEvent.click(screen.getByRole('button', { name: 'Adicionar credencial' }));
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'credential' },
  });
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'isolated test secret' },
  });
  expect(
    screen.getByRole('button', { name: 'Salvar credencial' }),
  ).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  fireEvent.click(
    within(
      await screen.findByRole('dialog', { name: 'Substituir acesso?' }),
    ).getByRole('button', { name: 'Substituir acesso' }),
  );
  await waitFor(() => expect(api.putCredential).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.putCredential).mock.calls[0][2].affected_links).toBe(2);
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(
    screen.queryByDisplayValue('isolated test secret'),
  ).not.toBeInTheDocument();
});
it.each(['OK', 'REVISION_CONFLICT'])(
  'credential selection is locked and late %s completion cannot affect a reopened form',
  async (code) => {
    let resolve!: (r: EquipmentResult) => void;
    vi.mocked(api.putCredential).mockImplementationOnce(
      () => new Promise((r) => (resolve = r)),
    );
    const view = render(
      <Equipment
        credentialsPage
        profile="credential-late"
        activation="surface"
        connected
      />,
    );
    await screen.findByRole('heading', { name: 'Credenciais' });
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Adicionar credencial' }),
      ).toBeEnabled(),
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Adicionar credencial' }),
    );
    fireEvent.change(screen.getByLabelText('Alias'), {
      target: { value: 'Read' },
    });
    fireEvent.change(screen.getByLabelText('Usuário remoto'), {
      target: { value: 'reader' },
    });
    fireEvent.change(screen.getByLabelText('Senha remota'), {
      target: { value: 'test' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
    expect(screen.getByLabelText('Credencial para editar')).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
    expect(
      screen.getByRole('dialog', { name: 'Credenciais de consulta' }),
    ).toBeInTheDocument();
    view.unmount();
    render(
      <Equipment
        credentialsPage
        profile="credential-late"
        activation="surface"
        connected
      />,
    );
    await screen.findByRole('heading', { name: 'Credenciais' });
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Adicionar credencial' }),
      ).toBeEnabled(),
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Adicionar credencial' }),
    );
    await act(async () => resolve({ ...data(), code }));
    expect(
      screen.getByRole('dialog', { name: 'Credenciais de consulta' }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Alias')).toHaveValue('');
    expect(screen.queryByText(/O cadastro mudou/)).not.toBeInTheDocument();
  },
);
it('a blocked vault explains the disabled direct form inside its dialog exactly once', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...data(),
    data: { ...data().data!, vault_available: false },
  });
  render(<Equipment profile="vault-failed" activation="a" connected />);
  await screen.findByText(
    'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
  );
  await beginAdd();
  expect(
    screen.getByText(
      'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
    ),
  ).toBeVisible();
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__add__' },
  });
  const dialog = screen.getByRole('dialog', {
    name: 'Credenciais de consulta',
  });
  expect(within(dialog).getByLabelText('Senha remota')).toBeDisabled();
  expect(
    within(dialog).getByRole('button', { name: 'Salvar credencial' }),
  ).toBeDisabled();
  expect(
    screen.getAllByText(
      'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
    ),
  ).toHaveLength(1);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Cancelar' }));
  expect(screen.getByLabelText('Credencial de consulta')).toBeEnabled();
});
it('Q03 updates the credential catalog after successful replacement and recovers catalog failure locally', async () => {
  vi.mocked(api.putCredential).mockResolvedValue(data());
  render(
    <Equipment
      credentialsPage
      profile="Q03-edit"
      activation="catalog"
      connected
    />,
  );
  await screen.findByRole('heading', { name: 'Credenciais' });
  fireEvent.click(screen.getByRole('button', { name: 'Adicionar credencial' }));
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'credential' },
  });
  fireEvent.change(screen.getByLabelText('Alias'), {
    target: { value: 'Updated catalog' },
  });
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic replacement secret' },
  });
  vi.mocked(api.credentials)
    .mockResolvedValueOnce({ ...data(), code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        credentials: [
          {
            id: 'credential',
            alias: 'Updated catalog',
            user: 'reader',
            revision: 2,
            links: 2,
          },
        ],
      },
    });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  fireEvent.click(
    within(
      await screen.findByRole('dialog', { name: 'Substituir acesso?' }),
    ).getByRole('button', { name: 'Substituir acesso' }),
  );
  await screen.findByRole('button', { name: 'Recarregar credenciais' });
  const inventoryCalls = vi.mocked(api.list).mock.calls.length;
  fireEvent.click(
    screen.getByRole('button', { name: 'Recarregar credenciais' }),
  );
  await waitFor(() => expect(api.credentials).toHaveBeenCalledTimes(3));
  fireEvent.click(screen.getByRole('button', { name: 'Adicionar credencial' }));
  expect(
    await screen.findByRole('option', { name: 'Updated catalog · 2 vínculos' }),
  ).toBeInTheDocument();
  expect(api.list).toHaveBeenCalledTimes(inventoryCalls);
  expect(api.start).not.toHaveBeenCalled();
});
it('credentials remain accessible for an empty company', async () => {
  vi.mocked(api.list).mockResolvedValue(data([]));
  render(
    <Equipment credentialsPage profile="X" activation="a" connected={true} />,
  );
  await screen.findByRole('heading', { name: 'Credenciais' });
  expect(
    screen.getAllByRole('button', { name: 'Adicionar credencial' }),
  ).toHaveLength(1);
  fireEvent.click(screen.getByRole('button', { name: 'Adicionar credencial' }));
  await screen.findByRole('dialog');
});
