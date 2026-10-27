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
import { Desktop, Statusbar } from './desktop';
import { equipmentAPI as api } from './equipment-api';
import type { EquipmentResult, Device } from './equipment-api';
vi.mock('./presentation', () => ({
  equipmentAPI: {
    list: vi.fn(),
    enrollmentStatus: vi.fn(),
    get: vi.fn(),
    save: vi.fn(),
    active: vi.fn(),
    credentials: vi.fn(),
    putCredential: vi.fn(),
    inspect: vi.fn(),
    confirm: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
  },
}));
const device = {
  id: 'device-X',
  role: 'concentrator',
  name: 'Router X',
  address: '127.0.0.1',
  port: 22,
  transport: 'ssh',
  credential_id: 'credential',
  revision: 1,
  active: true,
  fingerprint: '',
  trust_origin: '',
} as Device;
const data = (devices: Device[] = [device]) =>
  ({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { devices, credentials: [], next_cursor: 0, vault_available: true },
  }) as EquipmentResult;
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.list).mockResolvedValue(data());
  vi.mocked(api.get).mockImplementation(async (_p, _a, id) => {
    const latest = (await vi.mocked(api.list).mock.results.at(-1)?.value) as
      EquipmentResult | undefined;
    const target = latest?.data?.devices.find((d) => d.id === id) || device;
    return { ...data(), data: { ...data().data!, device: target } };
  });
  vi.mocked(api.credentials).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      credentials: [
        {
          id: 'credential',
          alias: 'Consulta',
          user: 'reader',
          revision: 1,
          links: 2,
        },
      ],
    },
  } as EquipmentResult);
});
for (const active of [true, false]) {
  for (const fingerprint of ['', 'SHA256:accepted']) {
    it(`saves an edited active=${active} pinned=${Boolean(fingerprint)} device without remote work`, async () => {
      const current = { ...device, active, fingerprint };
      vi.mocked(api.list).mockResolvedValue(data([current]));
      vi.mocked(api.save).mockResolvedValue({
        ...data(),
        data: {
          ...data().data!,
          device: { ...current, name: 'Edited name', revision: 2 },
        },
      });
      render(
        <Equipment profile="X" activation="a" connected createRequest={0} />,
      );
      await screen.findByRole('heading', { name: 'Router X' });
      fireEvent.click(
        screen.getByRole('button', { name: 'Editar equipamento' }),
      );
      fireEvent.change(screen.getByLabelText('Nome'), {
        target: { value: 'Edited name' },
      });
      fireEvent.click(
        screen.getByRole('button', { name: 'Salvar equipamento' }),
      );
      await waitFor(() =>
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
      );
      expect(api.save).toHaveBeenCalledExactlyOnceWith(
        'X',
        'a',
        expect.objectContaining({
          id: device.id,
          revision: 1,
          name: 'Edited name',
          credential_id: 'credential',
          address: '127.0.0.1',
        }),
      );
      await act(async () => {});
      expect(api.inspect).not.toHaveBeenCalled();
      expect(api.start).not.toHaveBeenCalled();
      expect(api.active).not.toHaveBeenCalled();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  }
}
for (const failure of ['SERVICE_UNAVAILABLE', 'reject']) {
  it(`retries an initial inventory ${failure} locally without claiming empty`, async () => {
    if (failure === 'reject')
      vi.mocked(api.list).mockRejectedValueOnce(new Error('transport'));
    else
      vi.mocked(api.list).mockResolvedValueOnce({ ...data(), code: failure });
    render(
      <Equipment profile="X" activation="a" connected createRequest={0} />,
    );
    await screen.findByRole('alert');
    expect(
      screen.queryByRole('heading', { name: 'Nenhum equipamento cadastrado' }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
    expect(
      await screen.findByRole('heading', { name: 'Router X' }),
    ).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(api.list).toHaveBeenCalledTimes(2);
    expect(api.start).not.toHaveBeenCalled();
    expect(api.inspect).not.toHaveBeenCalled();
  });
}
for (const changeSurface of [true, false]) {
  it(`keeps a late post-query failure on its original surface: changed=${changeSurface}`, async () => {
    let reject!: (reason: Error) => void;
    vi.mocked(api.get).mockImplementation(
      () =>
        new Promise((_resolve, fail) => {
          reject = fail;
        }),
    );
    const current = { ...device, fingerprint: 'SHA256:accepted' };
    vi.mocked(api.list).mockResolvedValue(data([current]));
    vi.mocked(api.start).mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'late-query',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      },
    });
    render(
      <Equipment profile="late" activation="a" connected createRequest={0} />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(
      screen.getByRole('button', { name: /^(Conectar|Consultar resumo)$/ }),
    );
    await waitFor(() => expect(api.get).toHaveBeenCalled());
    if (changeSurface) {
      fireEvent.click(
        screen.getByRole('button', { name: 'Editar equipamento' }),
      );
      fireEvent.change(screen.getByLabelText('Nome'), {
        target: { value: 'New draft' },
      });
    }
    await act(async () => reject(new Error('late transport failure')));
    if (changeSurface) {
      expect(screen.getByLabelText('Nome')).toHaveValue('New draft');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(
        screen.getByRole('dialog', { name: 'Editar equipamento' }),
      ).toBeVisible();
    } else
      expect(await screen.findByRole('alert')).toHaveTextContent('serviço');
  });
}
for (const path of ['target', 'detail']) {
  for (const outcome of ['success', 'reject']) {
    for (const changed of [true, false]) {
      it(`scopes ${path} read ${outcome} to its surface changed=${changed}`, async () => {
        let complete!: (r: EquipmentResult) => void;
        let reject!: (e: Error) => void;
        const updated = {
          ...data(),
          data: {
            ...data().data!,
            device: { ...device, name: 'Remote changed' },
          },
        };
        vi.mocked(api.get)
          .mockResolvedValue(updated)
          .mockImplementationOnce(
            () =>
              new Promise((resolve, fail) => {
                complete = resolve;
                reject = fail;
              }),
          );
        const props = {
          profile: 'X',
          activation: 'a',
          connected: true,
          createRequest: 0,
          targetID: path === 'target' ? device.id : undefined,
        };
        const view = render(<Equipment {...props} />);
        if (path === 'detail')
          fireEvent.click(await screen.findByRole('row', { name: /Router X/ }));
        await waitFor(() => expect(api.get).toHaveBeenCalledTimes(1));
        if (changed) {
          view.rerender(<Equipment {...props} createRequest={1} />);
          fireEvent.change(screen.getByLabelText('Nome'), {
            target: { value: 'Preserved draft' },
          });
        }
        await act(async () => {
          if (outcome === 'reject') reject(new Error('late read'));
          else complete(updated);
        });
        if (changed) {
          expect(screen.getByLabelText('Nome')).toHaveValue('Preserved draft');
          expect(screen.queryByRole('alert')).not.toBeInTheDocument();
          expect(
            screen.queryByRole('heading', {
              name: 'Remote changed',
              hidden: true,
            }),
          ).not.toBeInTheDocument();
        } else if (outcome === 'reject')
          expect(await screen.findByRole('alert')).toHaveTextContent('serviço');
        else
          expect(
            await screen.findByRole('heading', { name: 'Remote changed' }),
          ).toBeVisible();
      });
    }
  }
}
it('persists a real form request with stable operation ID and explicit company context', async () => {
  vi.mocked(api.save)
    .mockResolvedValueOnce({
      ...data(),
      code: 'REVISION_CONFLICT',
    })
    .mockResolvedValueOnce(data());
  render(<Equipment profile="X" activation="a" connected createRequest={1} />);
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'New destination' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '127.0.0.1' },
  });
  fireEvent.click(
    screen.getByRole('button', { name: 'Usar credencial existente' }),
  );
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: 'credential' },
  });
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
  const first = vi.mocked(api.save).mock.calls[0];
  expect(first[0]).toBe('X');
  expect(first[1]).toBe('a');
  expect(first[2].transport).toBe('ssh');
  expect(first[2].operation_id).toMatch(/^[a-f0-9-]{36}$/);
  await screen.findAllByText(/O cadastro mudou/);
  expect(
    screen.queryByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  ).not.toBeInTheDocument();
  vi.mocked(api.enrollmentStatus).mockResolvedValue({
    ...data(),
    data: { ...data().data!, operation_state: 'NOT_FOUND' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.save).mock.calls[1][2].operation_id).toBe(
    first[2].operation_id,
  );
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
});
it('shared credential replacement requires affected-link acknowledgement and clears typed secret', async () => {
  vi.mocked(api.putCredential).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  render(<Equipment profile="X" activation="a" connected createRequest={0} />);
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'credential' },
  });
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'isolated test secret' },
  });
  expect(
    screen.getByRole('button', { name: 'Salvar credencial' }),
  ).toBeDisabled();
  fireEvent.click(screen.getByRole('checkbox', { name: /2 equipamentos/ }));
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  await waitFor(() => expect(api.putCredential).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.putCredential).mock.calls[0][2].affected_links).toBe(2);
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(
    screen.queryByDisplayValue('isolated test secret'),
  ).not.toBeInTheDocument();
});
it('inactive filtering stays explicit; no remote work happens while listing or editing', async () => {
  render(<Equipment profile="X" activation="a" connected createRequest={0} />);
  await screen.findByRole('heading', { name: 'Router X' });
  expect(api.start).not.toHaveBeenCalled();
  expect(api.inspect).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('Estado dos equipamentos'), {
    target: { value: 'inactive' },
  });
  await waitFor(() =>
    expect(api.list).toHaveBeenLastCalledWith('X', 'a', '', 0, true),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
  expect(api.start).not.toHaveBeenCalled();
});
it('Q03 late X inventory and credential responses cannot populate the remounted Y surface', async () => {
  let resolve!: (r: EquipmentResult) => void;
  let credentialResolve!: (r: EquipmentResult) => void;
  vi.mocked(api.credentials)
    .mockImplementationOnce(
      () =>
        new Promise((r) => {
          credentialResolve = r;
        }),
    )
    .mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        credentials: [
          {
            id: 'fresh-y',
            alias: 'Fresh Y',
            user: 'reader-y',
            revision: 1,
            links: 0,
          },
        ],
      },
    });
  vi.mocked(api.list)
    .mockImplementationOnce(() => new Promise((r) => (resolve = r)))
    .mockResolvedValue(data([]));
  const view = render(
    <Equipment
      key="X"
      profile="X"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  view.rerender(
    <Equipment
      key="Y"
      profile="Y"
      activation="b"
      connected
      createRequest={0}
    />,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  await act(async () => {
    resolve(data());
    credentialResolve({
      ...data(),
      data: {
        ...data().data!,
        credentials: [
          {
            id: 'old-x',
            alias: 'Late X',
            user: 'reader-x',
            revision: 1,
            links: 0,
          },
        ],
      },
    });
  });
  expect(screen.queryByText('Router X')).not.toBeInTheDocument();
  expect(api.credentials).toHaveBeenCalledTimes(2);
  expect(api.credentials).toHaveBeenCalledWith('X', 'a', 0);
  expect(api.credentials).toHaveBeenCalledWith('Y', 'b', 0);
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Usar credencial existente' }),
  );
  expect(
    screen.getByRole('option', { name: 'Fresh Y · reader-y' }),
  ).toBeInTheDocument();
  expect(screen.queryByText(/Late X/)).not.toBeInTheDocument();
});
it('first acceptance is deliberate without an invented origin and failed acceptance cannot authenticate', async () => {
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: {
        id: 'j',
        device_id: device.id,
        kind: 'trust.inspect',
        state: 'SUCCEEDED',
        code: 'OK',
        fingerprint: 'SHA256:fixture',
      },
    },
  } as EquipmentResult);
  vi.mocked(api.confirm).mockResolvedValue({
    ...data(),
    code: 'TRUST_CHANGED',
  });
  render(<Equipment profile="X" activation="a" connected createRequest={0} />);
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByRole('dialog', { name: 'Primeira conexão' });
  expect(api.start).not.toHaveBeenCalled();
  expect(
    screen.getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  ).toBeEnabled();
  expect(
    screen.queryByLabelText('Origem da conferência'),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  );
  await waitFor(() => expect(api.confirm).toHaveBeenCalledTimes(1));
  expect(
    await within(screen.getByRole('dialog')).findByRole('alert'),
  ).toHaveTextContent('A chave do destino mudou');
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  expect(api.start).not.toHaveBeenCalled();
});
it('navigation opens into sidebar and returns focus after selection and Escape', () => {
  const navigate = vi.fn();
  render(
    <Desktop
      page={3}
      navigate={navigate}
      brand={{ name: 'X', accent: 'teal', logo: '' }}
      account="operator"
      theme="light"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="X"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
    />,
  );
  const trigger = screen.getByRole('button', { name: 'Navegação' });
  fireEvent.click(trigger);
  expect(
    screen.getByRole('button', { name: 'Selecionar empresa' }),
  ).toHaveFocus();
  fireEvent.click(screen.getByRole('button', { name: 'Equipamentos' }));
  expect(trigger).toHaveFocus();
  expect(navigate).toHaveBeenCalledWith(4);
  fireEvent.click(trigger);
  fireEvent.keyDown(
    screen.getByRole('button', { name: 'Selecionar empresa' }),
    { key: 'Escape' },
  );
  expect(trigger).toHaveFocus();
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
  fireEvent.click(screen.getByRole('button', { name: 'Aparência' }));
  expect(trigger).toHaveFocus();
  expect(navigate).toHaveBeenCalledWith(7);
});
it('an uncertain query start reuses its ID and never retries authentication by itself', async () => {
  const trusted = { ...device, fingerprint: 'SHA256:fixture' };
  vi.mocked(api.list).mockResolvedValue(data([trusted]));
  vi.mocked(api.start)
    .mockResolvedValueOnce({ ...data(), code: 'DEADLINE_EXCEEDED' })
    .mockResolvedValueOnce({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'job',
          device_id: device.id,
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      },
    });
  render(<Equipment profile="X" activation="a" connected createRequest={0} />);
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo da operação/);
  expect(api.start).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.start).mock.calls[0][3]).toBe(
    vi.mocked(api.start).mock.calls[1][3],
  );
});
it('full detail interfaces survive a subsequent list projection and a new context discards late detail', async () => {
  const observation = {
    identity: 'Fixture router',
    version: '7.26',
    family: 'v7',
    board: 'Board',
    uptime: '1h',
    cpu_percent: 12,
    free_memory_bytes: 100,
    total_memory_bytes: 200,
    interfaces: [
      { name: 'ether1', kind: 'ether', running: true, disabled: false },
    ],
    observed_at: '2026-10-24T14:00:00Z',
    source: 'SSH fixture',
    quality: 'complete',
  };
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, result: { ...observation, interfaces: [] } }]),
  );
  vi.mocked(api.get).mockResolvedValue({
    ...data(),
    data: { ...data().data!, device: { ...device, result: observation } },
  });
  const view = render(
    <Equipment
      key="X"
      profile="X"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Abrir equipamento' }));
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces físicas' }),
  );
  await screen.findByText('ether1');
  fireEvent.click(
    screen.getByRole('button', { name: 'Voltar aos equipamentos' }),
  );
  fireEvent.change(screen.getByLabelText('Buscar equipamento'), {
    target: { value: 'Router' },
  });
  await waitFor(() =>
    expect(api.list).toHaveBeenLastCalledWith('X', 'a', 'Router', 0, false),
  );
  expect(screen.getByText('ether1')).toBeInTheDocument();
  let resolve!: (r: EquipmentResult) => void;
  vi.mocked(api.get).mockImplementationOnce(
    () => new Promise((r) => (resolve = r)),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Abrir equipamento' }));
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(
    <Equipment
      key="Y"
      profile="Y"
      activation="b"
      connected
      createRequest={0}
    />,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  await act(async () =>
    resolve({
      ...data(),
      data: { ...data().data!, device: { ...device, result: observation } },
    }),
  );
  expect(screen.queryByText('ether1')).not.toBeInTheDocument();
});
it('clearing a consumed create request during navigation never opens a new form', async () => {
  const view = render(
    <Equipment profile="X" activation="a" connected createRequest={1} />,
  );
  await screen.findByRole('dialog', { name: 'Adicionar equipamento' });
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  view.rerender(
    <Equipment profile="X" activation="a" connected createRequest={0} />,
  );
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

it.each(['OK', 'REVISION_CONFLICT'])(
  'late %s save cannot close or mark a reopened dialog',
  async (code) => {
    let resolve!: (r: EquipmentResult) => void;
    vi.mocked(api.save).mockImplementationOnce(
      () => new Promise((r) => (resolve = r)),
    );
    const view = render(
      <Equipment
        profile="late"
        activation="surface"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
    fireEvent.click(
      screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
    );
    await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
    expect(
      screen.getByRole('dialog', { name: 'Editar equipamento' }),
    ).toBeInTheDocument();
    view.unmount();
    render(
      <Equipment
        profile="late"
        activation="surface"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
    await act(async () => resolve({ ...data(), code }));
    expect(
      screen.getByRole('dialog', { name: 'Editar equipamento' }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Nome')).toHaveValue('Router X');
    expect(screen.queryByText(/O cadastro mudou/)).not.toBeInTheDocument();
    if (code === 'OK')
      await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  },
);
it('late detail cannot restore an old selected device', async () => {
  const other = { ...device, id: 'other', name: 'Router B' };
  vi.mocked(api.list).mockResolvedValue(
    data([
      { ...device, result: { observed_at: '2026-10-24' } as Device['result'] },
      other,
    ]),
  );
  let resolve!: (r: EquipmentResult) => void;
  vi.mocked(api.get).mockImplementationOnce(
    () => new Promise((r) => (resolve = r)),
  );
  render(
    <Equipment
      profile="detail"
      activation="surface"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Abrir equipamento' }));
  if (screen.queryByRole('button', { name: 'Voltar aos equipamentos' }))
    fireEvent.click(
      screen.getByRole('button', { name: 'Voltar aos equipamentos' }),
    );
  fireEvent.click(screen.getByRole('row', { name: /Router B/ }));
  await act(async () =>
    resolve({ ...data(), data: { ...data().data!, device } }),
  );
  expect(screen.getByRole('heading', { name: 'Router B' })).toBeInTheDocument();
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
        profile="credential-late"
        activation="surface"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
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
        profile="credential-late"
        activation="surface"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
    await act(async () => resolve({ ...data(), code }));
    expect(
      screen.getByRole('dialog', { name: 'Credenciais de consulta' }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Alias')).toHaveValue('');
    expect(screen.queryByText(/O cadastro mudou/)).not.toBeInTheDocument();
  },
);
const runningJob = {
  id: 'poll-job',
  device_id: device.id,
  kind: 'device.query-start',
  state: 'RUNNING',
  code: 'OK',
};
it.each(['reject', 'reply'])(
  'polling recovers from %s without another start',
  async (kind) => {
    vi.mocked(api.list).mockResolvedValue(
      data([{ ...device, fingerprint: 'SHA256:test' }]),
    );
    vi.mocked(api.start).mockResolvedValue({
      ...data(),
      data: { ...data().data!, job: runningJob },
    } as EquipmentResult);
    if (kind === 'reject')
      vi.mocked(api.status).mockRejectedValueOnce(new Error('transport'));
    else
      vi.mocked(api.status).mockResolvedValueOnce({
        ...data(),
        code: 'SERVICE_UNAVAILABLE',
      });
    vi.mocked(api.status).mockResolvedValueOnce({
      ...data(),
      data: { ...data().data!, job: { ...runningJob, state: 'SUCCEEDED' } },
    } as EquipmentResult);
    render(
      <Equipment
        profile={'poll-' + kind}
        activation="surface"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    );
    await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByText(/Consulta SSH em curso/),
      ).not.toBeInTheDocument(),
    );
    expect(api.start).toHaveBeenCalledTimes(1);
  },
);
it('exhausted status recovery unlocks UI and deliberate retry reconciles the same start ID', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.status).mockRejectedValue(new Error('transport'));
  render(
    <Equipment
      profile="poll-exhaust"
      activation="surface"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(3));
  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    ).toBeEnabled(),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(api.start).mock.calls[0][3]).toBe(
    vi.mocked(api.start).mock.calls[1][3],
  );
});
it('uncertain IDs survive remount and query/inspect/device alternation; interrupted receipt requires deliberate new start', async () => {
  const trusted = { ...device, fingerprint: 'SHA256:test' },
    other = { ...trusted, id: 'other', name: 'Router B' };
  vi.mocked(api.list).mockResolvedValue(data([trusted, other]));
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  const mount = () =>
    render(
      <Equipment
        profile="retain"
        activation="surface"
        connected
        createRequest={0}
      />,
    );
  const view = mount();
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo/);
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await waitFor(() => expect(api.inspect).toHaveBeenCalledTimes(1));
  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
      }),
    ).toBeEnabled(),
  );
  if (screen.queryByRole('button', { name: 'Voltar aos equipamentos' }))
    fireEvent.click(
      screen.getByRole('button', { name: 'Voltar aos equipamentos' }),
    );
  fireEvent.click(screen.getByRole('row', { name: /Router B/ }));
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  view.unmount();
  mount();
  await screen.findByRole('heading', { name: 'Router X' });
  vi.mocked(api.start).mockResolvedValueOnce({
    ...data(),
    code: 'JOB_INTERRUPTED',
  });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/interrompida ou expirou/);
  expect(vi.mocked(api.start).mock.calls[2][3]).toBe(
    vi.mocked(api.start).mock.calls[0][3],
  );
  expect(api.start).toHaveBeenCalledTimes(3);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(4));
  expect(vi.mocked(api.start).mock.calls[3][3]).not.toBe(
    vi.mocked(api.start).mock.calls[0][3],
  );
});

it('neutral footer remains truthful after a reading, failed refresh, navigation and context change', async () => {
  const observation = {
    identity: 'Last valid reading',
    version: '7',
    family: 'v7',
    board: 'B',
    uptime: '1h',
    cpu_percent: 1,
    free_memory_bytes: 1,
    total_memory_bytes: 2,
    interfaces: [],
    observed_at: '2026-10-24T14:00:00Z',
    source: 'SSH',
    quality: 'complete',
  };
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, result: observation }]),
  );
  const view = render(
    <>
      <Statusbar connected />
      <Equipment
        key="X"
        profile="X-footer"
        activation="footer"
        connected
        createRequest={0}
      />
    </>,
  );
  await screen.findByRole('heading', { name: 'Last valid reading' });
  expect(screen.getByText('Inventário por empresa')).toBeInTheDocument();
  expect(
    screen.getByText('Consultas sob demanda · Equipamentos'),
  ).toBeInTheDocument();
  vi.mocked(api.list).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  fireEvent.change(screen.getByLabelText('Buscar equipamento'), {
    target: { value: 'refresh' },
  });
  await screen.findByText(/O serviço está indisponível/);
  expect(
    screen.getByRole('heading', { name: 'Last valid reading' }),
  ).toBeInTheDocument();
  view.rerender(<Statusbar connected />);
  expect(screen.queryByText('Sem coletas')).not.toBeInTheDocument();
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(
    <>
      <Statusbar connected />
      <Equipment
        key="Y"
        profile="Y-footer"
        activation="other"
        connected
        createRequest={0}
      />
    </>,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.queryByText('Rede não consultada')).not.toBeInTheDocument();
  expect(
    screen.getByText('Consultas sob demanda · Equipamentos'),
  ).toBeInTheDocument();
});
it('an uncertain inspection retains its own ID across unmount and another operation kind', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.inspect)
    .mockRejectedValueOnce(new Error('lost response'))
    .mockResolvedValueOnce({ ...data(), code: 'JOB_INTERRUPTED' });
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'DEADLINE_EXCEEDED',
  });
  const view = render(
    <Equipment
      profile="retain-inspect"
      activation="surface"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByText(/O serviço está indisponível/);
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  await screen.findByText(/O prazo da operação/);
  view.unmount();
  render(
    <Equipment
      profile="retain-inspect"
      activation="surface"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  await screen.findByText(/interrompida ou expirou/);
  expect(api.inspect).toHaveBeenCalledTimes(2);
  expect(vi.mocked(api.inspect).mock.calls[0][3]).toBe(
    vi.mocked(api.inspect).mock.calls[1][3],
  );
  expect(api.start).toHaveBeenCalledTimes(1);
});
it('cancel reconciles the running job without another authentication', async () => {
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, fingerprint: 'SHA256:test' }]),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.status).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job: runningJob },
  } as EquipmentResult);
  vi.mocked(api.cancel).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: { ...runningJob, state: 'CANCELLED', code: 'CANCELLED' },
    },
  } as EquipmentResult);
  render(
    <Equipment
      profile="cancel-status"
      activation="surface"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', {
      name: /^(Conectar|Atualizar|Consultar resumo|Atualizar resumo)$/,
    }),
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Cancelar consulta' }),
  );
  await waitFor(() =>
    expect(screen.queryByText(/Consulta SSH em curso/)).not.toBeInTheDocument(),
  );
  expect(api.cancel).toHaveBeenCalledWith(
    'cancel-status',
    'surface',
    'poll-job',
  );
  expect(api.start).toHaveBeenCalledTimes(1);
});

it('first enrollment is direct and the initial empty state has exactly one CTA without irrelevant controls', async () => {
  vi.mocked(api.list).mockResolvedValue(data([]));
  vi.mocked(api.credentials).mockResolvedValue(data([]));
  render(<Equipment profile="X" activation="a" connected createRequest={0} />);
  await screen.findByRole('heading', { name: 'Nenhum equipamento cadastrado' });
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
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
  expect(screen.getByLabelText('Usuário')).toBeInTheDocument();
  expect(screen.getByLabelText('Senha')).toHaveAttribute('type', 'password');
  expect(screen.queryByLabelText('Alias')).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Usar credencial existente' }),
  ).toBeInTheDocument();
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
  render(
    <Equipment
      profile="direct-lost"
      activation="a"
      connected
      createRequest={1}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  for (const [label, value] of [
    ['Nome', 'New direct'],
    ['IP de gerenciamento', '127.0.0.1'],
    ['Usuário', 'reader'],
    ['Senha', 'transient fixture secret'],
  ]) {
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  }
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
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
    screen.queryByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
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
  render(
    <Equipment
      profile="direct-retry"
      activation="a"
      connected
      createRequest={1}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
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
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Conferir cadastro' }));
  await screen.findByText(/Para tentar com o mesmo cadastro/);
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(screen.getByLabelText('Senha')).toHaveValue('');
  fill();
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await waitFor(() => expect(sent).toHaveLength(2));
  expect(sent[1].id).toBe(sent[0].id);
  expect(sent[1].operation_id).toBe(sent[0].operation_id);
});

it('no active devices keeps inactive filtering, while an empty search is recoverable without a table or inspector', async () => {
  vi.mocked(api.list).mockImplementation(
    async (_p, _a, search, _cursor, inactive) => ({
      ...data(search || !inactive ? [] : [{ ...device, active: false }]),
      data: {
        ...data(search || !inactive ? [] : [{ ...device, active: false }])
          .data!,
        total_devices: 1,
      },
    }),
  );
  render(
    <Equipment
      profile="inactive-only"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Nenhum equipamento ativo' });
  expect(screen.getByLabelText('Estado dos equipamentos')).toBeInTheDocument();
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
  fireEvent.click(screen.getByRole('button', { name: 'Mostrar inativos' }));
  await screen.findByRole('button', { name: 'Reativar' });
  fireEvent.change(screen.getByLabelText('Buscar equipamento'), {
    target: { value: 'absent' },
  });
  await screen.findByRole('heading', { name: 'Nenhum resultado' });
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(screen.queryByText('Detalhes do equipamento')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar busca' }));
  await screen.findByRole('button', { name: 'Reativar' });
});

it('the full desktop owns only one Add action in empty and populated inventory states', async () => {
  const view = render(
    <Desktop
      page={4}
      navigate={() => {}}
      brand={{ name: '', accent: 'teal', logo: '' }}
      account="Fixture"
      theme="light"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="X"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
      inventory={
        <Equipment profile="cta" activation="a" connected createRequest={0} />
      }
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(
    <Desktop
      page={4}
      navigate={() => {}}
      brand={{ name: '', accent: 'teal', logo: '' }}
      account="Fixture"
      theme="dark"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="Y"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
      inventory={
        <Equipment
          key="empty"
          profile="cta-empty"
          activation="b"
          connected
          createRequest={0}
        />
      }
    />,
  );
  await screen.findByRole('heading', { name: 'Nenhum equipamento cadastrado' });
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
});

it('an uncertain enrollment never lends its operation ID to an unrelated edit', async () => {
  const sent: Record<string, unknown>[] = [];
  vi.mocked(api.save).mockImplementation(async (_p, _a, d) => {
    sent.push(structuredClone(d));
    return { ...data(), code: 'SERVICE_UNAVAILABLE' };
  });
  render(
    <Equipment
      profile="pending-edit"
      activation="a"
      connected
      createRequest={1}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  for (const [label, value] of [
    ['Nome', 'Uncertain'],
    ['IP de gerenciamento', '127.0.0.1'],
    ['Usuário', 'reader'],
    ['Senha', 'fixture only'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
  expect(screen.queryByLabelText('Senha')).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await waitFor(() => expect(sent).toHaveLength(2));
  expect(sent[1].operation_id).not.toBe(sent[0].operation_id);
  expect(sent[1]).toMatchObject({
    id: device.id,
    revision: 1,
    credential_id: device.credential_id,
    secret: '',
    user: '',
  });
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
  render(
    <Equipment
      profile="reuse-lost"
      activation="a"
      connected
      createRequest={1}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', { name: 'Usar credencial existente' }),
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'Reuse' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '127.0.0.1' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: 'credential' },
  });
  fireEvent.click(
    screen.getByRole('button', { name: /^Salvar (equipamento|e conectar)$/ }),
  );
  await screen.findByRole('button', { name: 'Conferir cadastro' });
  expect(sent).toMatchObject({
    credential_id: 'credential',
    user: '',
    secret: '',
  });
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
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
  render(
    <Equipment
      profile="fields-red"
      activation="a"
      connected
      createRequest={1}
    />,
  );
  await screen.findByRole('dialog', { name: 'Adicionar equipamento' });
  expect(screen.getByLabelText('Usuário')).toBeInTheDocument();
  expect(screen.getByLabelText('Senha')).toHaveAttribute('type', 'password');
  expect(screen.queryByLabelText('Alias')).not.toBeInTheDocument();
});

it('a blocked vault explains the disabled direct form inside its dialog exactly once', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...data(),
    data: { ...data().data!, vault_available: false },
  });
  render(
    <Equipment
      profile="vault-failed"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  await screen.findByText(
    'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
  );
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
  const dialog = screen.getByRole('dialog', { name: 'Adicionar equipamento' });
  expect(
    within(dialog).getByText(
      'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
    ),
  ).toBeInTheDocument();
  expect(
    screen.getAllByText(
      'Cofre indisponível. Confira a credencial do serviço nesta instalação.',
    ),
  ).toHaveLength(1);
  expect(within(dialog).getByLabelText('Senha')).toBeDisabled();
  expect(
    within(dialog).getByRole('button', {
      name: /^Salvar (equipamento|e conectar)$/,
    }),
  ).toBeDisabled();
  fireEvent.click(
    within(dialog).getByRole('button', { name: 'Usar credencial existente' }),
  );
  expect(within(dialog).getByLabelText('Credencial de consulta')).toBeEnabled();
});

it.each(['OK', 'TRUST_CHANGED'])(
  'late inspection status %s cannot open a second dialog or place an error on a newer form',
  async (code) => {
    let resolve!: (r: EquipmentResult) => void;
    vi.mocked(api.inspect).mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        job: {
          id: 'late-inspect-job',
          device_id: device.id,
          kind: 'trust.inspect',
          state: 'RUNNING',
          code: 'OK',
        },
      },
    });
    vi.mocked(api.status).mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    render(
      <Equipment
        profile={'inspect-form-' + code}
        activation="a"
        connected
        createRequest={0}
      />,
    );
    await screen.findByRole('heading', { name: 'Router X' });
    fireEvent.click(
      screen.getByRole('button', { name: 'Verificar identidade' }),
    );
    await waitFor(() => expect(api.status).toHaveBeenCalledTimes(1));
    fireEvent.click(
      screen.getByRole('button', { name: 'Adicionar equipamento' }),
    );
    await screen.findByRole('dialog', { name: 'Adicionar equipamento' });
    await act(async () =>
      resolve({
        ...data(),
        data: {
          ...data().data!,
          job: {
            id: 'late-inspect-job',
            device_id: device.id,
            kind: 'trust.inspect',
            state: code === 'OK' ? 'SUCCEEDED' : 'FAILED',
            code,
            fingerprint: code === 'OK' ? 'SHA256:fixture' : undefined,
          },
        },
      }),
    );
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(
      screen.getByRole('dialog', { name: 'Adicionar equipamento' }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/A chave do destino mudou/),
    ).not.toBeInTheDocument();
    expect(api.confirm).not.toHaveBeenCalled();
    expect(api.start).not.toHaveBeenCalled();
  },
);

it('completed inspection polling on its original surface still offers deliberate trust confirmation', async () => {
  const job = {
    id: 'original-inspect-job',
    device_id: device.id,
    kind: 'trust.inspect',
    state: 'RUNNING',
    code: 'OK',
  };
  vi.mocked(api.inspect).mockResolvedValue({
    ...data(),
    data: { ...data().data!, job },
  });
  vi.mocked(api.status).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      job: { ...job, state: 'SUCCEEDED', fingerprint: 'SHA256:fixture' },
    },
  });
  render(
    <Equipment
      profile="inspect-original"
      activation="a"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Verificar identidade' }));
  const panel = await screen.findByRole('dialog', {
    name: 'Primeira conexão',
  });
  expect(screen.getAllByRole('dialog')).toHaveLength(1);
  expect(within(panel).getByText('SHA256:fixture')).toBeInTheDocument();
  expect(
    within(panel).getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  ).toBeEnabled();
  expect(api.start).not.toHaveBeenCalled();
  expect(api.confirm).not.toHaveBeenCalled();
});

it('Q01 retires abandoned activations, preserves same-activation uncertain IDs and ignores late starts', async () => {
  const current = { ...device, fingerprint: 'SHA256:accepted' };
  vi.mocked(api.list).mockResolvedValue(data([current]));
  vi.mocked(api.start).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  const props = {
    profile: 'Q01',
    activation: 'a0',
    connected: true,
    createRequest: 0,
  };
  const view = render(<Equipment {...props} />);
  for (let i = 0; i < 130; i++) {
    view.rerender(<Equipment {...props} activation={'a' + i} />);
    const connect = await screen.findByRole('button', {
      name: /^(Conectar|Consultar resumo)$/,
    });
    await act(async () => {
      fireEvent.click(connect);
    });
    expect(api.start).toHaveBeenCalledTimes(i + 1);
    expect(connect).toBeEnabled();
  }
  const operation = vi.mocked(api.start).mock.calls.at(-1)![3];
  view.unmount();
  render(<Equipment {...props} activation="a129" />);
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(
    screen.getByRole('button', { name: /^(Conectar|Consultar resumo)$/ }),
  );
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(131));
  expect(vi.mocked(api.start).mock.calls.at(-1)![3]).toBe(operation);
  expect(api.inspect).not.toHaveBeenCalled();
});

it('Q03 inventory filters and refresh do not reload an independent credential catalog', async () => {
  render(
    <Equipment profile="Q03" activation="q03" connected createRequest={0} />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  await waitFor(() => expect(api.credentials).toHaveBeenCalledTimes(1));
  const search = screen.getByPlaceholderText(/Buscar/);
  fireEvent.change(search, { target: { value: 'Router' } });
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  expect(api.credentials).toHaveBeenCalledTimes(1);
});

it('Q03 updates the credential catalog after successful replacement and recovers catalog failure locally', async () => {
  vi.mocked(api.putCredential).mockResolvedValue(data());
  render(
    <Equipment
      profile="Q03-edit"
      activation="catalog"
      connected
      createRequest={0}
    />,
  );
  await screen.findByRole('heading', { name: 'Router X' });
  fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'credential' },
  });
  fireEvent.change(screen.getByLabelText('Alias'), {
    target: { value: 'Updated catalog' },
  });
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic replacement secret' },
  });
  fireEvent.click(screen.getByRole('checkbox', { name: /2 equipamentos/ }));
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
  await screen.findByRole('button', { name: 'Recarregar credenciais' });
  const inventoryCalls = vi.mocked(api.list).mock.calls.length;
  fireEvent.click(
    screen.getByRole('button', { name: 'Recarregar credenciais' }),
  );
  await waitFor(() => expect(api.credentials).toHaveBeenCalledTimes(3));
  fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
  expect(
    await screen.findByRole('option', { name: 'Updated catalog · 2 vínculos' }),
  ).toBeInTheDocument();
  expect(api.list).toHaveBeenCalledTimes(inventoryCalls);
  expect(api.start).not.toHaveBeenCalled();
});

it('L06C: credentials remain accessible for an empty company', async () => {
  vi.mocked(api.list).mockResolvedValue(data([]));
  render(
    <Equipment profile="X" activation="a" connected={true} createRequest={0} />,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.getAllByRole('button', { name: 'Credenciais' })).toHaveLength(
    1,
  );
  fireEvent.click(screen.getByRole('button', { name: 'Credenciais' }));
  await screen.findByRole('dialog');
});
