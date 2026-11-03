import { RadioCodes, RadioRoles, RadioTransports } from './radio-contract';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioEquipment } from './radio';
import { radioAPI as api } from './radio-api';
import type { Device } from './equipment-api';
import { equipmentAPI } from './equipment-api';
import { device, response, link, reading } from './radio-test-fixtures';

vi.mock('./radio-api', () => ({
  radioAPI: {
    current: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    confirmFamily: vi.fn(),
    link: vi.fn(),
    confirmLink: vi.fn(),
  },
}));

vi.mock('./equipment-api', () => ({
  equipmentMessage: (c: string) => c,
  equipmentAPI: {
    list: vi.fn(),
    get: vi.fn(),
    credentials: vi.fn(),
    save: vi.fn(),
  },
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(api.link).mockResolvedValue(response({ link }));
});

it('credential refusal opens a dropdown and saves the explicit choice without sending another query', async () => {
  const { target } = prepareExplicitCredentialChoiceResponses(
    prepareExplicitCredentialChoiceTarget(),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={target}
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  const dropdown = await screen.findByRole('combobox', {
    name: 'Outra credencial do equipamento',
  });
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  expect(screen.queryByRole('status')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Reabrir rádio armazenado' }),
  ).not.toBeInTheDocument();
  fireEvent.change(dropdown, { target: { value: 'chosen' } });
  await waitFor(() =>
    expect(equipmentAPI.save).toHaveBeenCalledWith(
      'company',
      'active',
      expect.objectContaining({
        id: target.id,
        credential_id: 'chosen',
        revision: 3,
      }),
    ),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.confirmFamily).not.toHaveBeenCalled();
});

it('keeps the newer selected credential when an older device read arrives late', async () => {
  const { old, data, resolveOld, stale } = prepareStaleCredentialReadResult(
    prepareStaleCredentialReadResponses(prepareStaleCredentialReadTarget()),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={old}
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  await waitFor(() => expect(equipmentAPI.get).toHaveBeenCalledTimes(1));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  const dropdown = await screen.findByRole('combobox', {
    name: 'Outra credencial do equipamento',
  });
  fireEvent.change(dropdown, { target: { value: 'chosen' } });
  await waitFor(() => expect(equipmentAPI.save).toHaveBeenCalledTimes(1));
  await act(async () => {
    resolveOld(data(old));
    await stale;
  });
  await waitFor(() =>
    expect(
      screen.queryByRole('combobox', {
        name: 'Outra credencial do equipamento',
      }),
    ).not.toBeInTheDocument(),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Consultar rádio' }));
  await waitFor(() =>
    expect(api.start).toHaveBeenLastCalledWith(
      'company',
      'active',
      expect.objectContaining({ revision: 4 }),
    ),
  );
});

function prepareExplicitCredentialChoiceTarget() {
  const target = {
    ...device,
    name: 'AP sintético',
    address: '192.0.2.10',
    port: 22,
    credential_id: 'refused',
    transport: RadioTransports.ssh,
    trust_origin: 'automatic-tofu',
  } as Device;
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'radiojob', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      ap_code: RadioCodes.REMOTE_AUTH_FAILED,
      job: {
        id: 'radiojob',
        state: 'FAILED',
        code: RadioCodes.REMOTE_AUTH_FAILED,
      },
    }),
  );
  vi.mocked(equipmentAPI.get).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      device: target,
    },
  });
  return { target };
}

function prepareExplicitCredentialChoiceResponses(
  context: ReturnType<typeof prepareExplicitCredentialChoiceTarget>,
) {
  const { target } = context;
  vi.mocked(equipmentAPI.credentials).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [
        {
          id: 'refused',
          alias: 'Recusada',
          user: 'reader',
          revision: 1,
          links: 1,
        },
        {
          id: 'chosen',
          alias: 'Escolhida',
          user: 'reader',
          revision: 1,
          links: 0,
        },
      ],
      next_cursor: 0,
      vault_available: true,
    },
  });
  vi.mocked(equipmentAPI.save).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      device: { ...target, credential_id: 'chosen', revision: 4 },
    },
  });
  return { target };
}

function prepareStaleCredentialReadTarget() {
  const old = {
    ...device,
    name: 'PTP',
    address: '192.0.2.20',
    port: 22,
    role: RadioRoles.ptp,
    credential_id: 'refused',
    transport: RadioTransports.ssh,
  } as Device;
  const data = (d: Device) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      device: d,
    },
  });
  let resolveOld!: (v: ReturnType<typeof data>) => void;
  const stale = new Promise<ReturnType<typeof data>>((resolve) => {
    resolveOld = resolve;
  });
  vi.mocked(equipmentAPI.get)
    .mockReturnValueOnce(stale)
    .mockResolvedValue(data(old));
  return { old, data, resolveOld, stale };
}

function prepareStaleCredentialReadResponses(
  context: ReturnType<typeof prepareStaleCredentialReadTarget>,
) {
  const { old, data, resolveOld, stale } = context;
  vi.mocked(equipmentAPI.credentials).mockResolvedValue({
    ...data(old),
    data: {
      ...data(old).data,
      credentials: [
        {
          id: 'refused',
          alias: 'Recusada',
          user: 'reader',
          revision: 1,
          links: 1,
        },
        {
          id: 'chosen',
          alias: 'Escolhida',
          user: 'reader',
          revision: 1,
          links: 0,
        },
      ],
    },
  });
  vi.mocked(equipmentAPI.save).mockResolvedValue(
    data({ ...old, revision: 4, credential_id: 'chosen' }),
  );
  vi.mocked(api.start)
    .mockResolvedValueOnce(
      response({ job: { id: 'first', state: 'RUNNING', code: 'OK' } }),
    )
    .mockResolvedValueOnce(
      response({ job: { id: 'second', state: 'RUNNING', code: 'OK' } }),
    )
    .mockResolvedValue(
      response({ job: { id: 'third', state: 'RUNNING', code: 'OK' } }),
    );
  return { old, data, resolveOld, stale };
}

function prepareStaleCredentialReadResult(
  context: ReturnType<typeof prepareStaleCredentialReadResponses>,
) {
  const { old, data, resolveOld, stale } = context;
  vi.mocked(api.status)
    .mockResolvedValueOnce(
      response({ job: { id: 'first', state: 'SUCCEEDED', code: 'OK' } }),
    )
    .mockResolvedValueOnce(
      response({
        job: {
          id: 'second',
          state: 'FAILED',
          code: RadioCodes.REMOTE_AUTH_FAILED,
        },
      }),
    );
  return { old, data, resolveOld, stale };
}

it('offers explicit credential recovery when a preserved stale reading makes the refused job partial', async () => {
  const target = preparePreservedCredentialRefusal();
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={target}
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  const dropdown = await screen.findByRole('combobox', {
    name: 'Outra credencial do equipamento',
  });
  expect(
    screen.getByText(
      'Leitura de um cadastro anterior; não confirma o vínculo atual.',
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/A última leitura boa conserva a data abaixo/),
  ).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Reabrir rádio armazenado' }),
  ).toBeInTheDocument();
  expect(screen.getByRole('alert')).toHaveTextContent(
    RadioCodes.REMOTE_AUTH_FAILED,
  );
  fireEvent.change(dropdown, { target: { value: 'chosen' } });
  await waitFor(() =>
    expect(equipmentAPI.save).toHaveBeenCalledWith(
      'company',
      'active',
      expect.objectContaining({
        id: target.id,
        credential_id: 'chosen',
        revision: 3,
      }),
    ),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.confirmFamily).not.toHaveBeenCalled();
});

it('preserves a partial cached reading without offering credentials when no authentication was refused', async () => {
  const { target } = prepareExplicitCredentialChoiceResponses(
    prepareExplicitCredentialChoiceTarget(),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      reading: { ...reading, perspective: 'CPE', clients: [] },
      ap_code: 'PARTIAL',
      job: { id: 'radiojob', state: 'FAILED', code: 'PARTIAL' },
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={target}
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  await screen.findByRole('alert');
  expect(
    screen.queryByRole('combobox', { name: 'Outra credencial do equipamento' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Escolher outra' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Reabrir rádio armazenado' }),
  ).toBeInTheDocument();
  expect(equipmentAPI.credentials).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(1);
});

it('does not reuse a preserved refusal code during a new running or successful query', async () => {
  const { target } = prepareExplicitCredentialChoiceResponses(
    prepareExplicitCredentialChoiceTarget(),
  );
  vi.mocked(api.current).mockResolvedValue(
    response({ reading, ap_code: RadioCodes.REMOTE_AUTH_FAILED }),
  );
  vi.mocked(api.status)
    .mockResolvedValueOnce(
      response({ job: { id: 'radiojob', state: 'RUNNING', code: 'OK' } }),
    )
    .mockResolvedValue(
      response({ job: { id: 'radiojob', state: 'SUCCEEDED', code: 'OK' } }),
    );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={target}
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar rádio' }),
  );
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(1));
  expect(
    screen.queryByRole('combobox', { name: 'Outra credencial do equipamento' }),
  ).not.toBeInTheDocument();
  expect(equipmentAPI.credentials).not.toHaveBeenCalled();
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
  await screen.findByRole('button', { name: 'Consultar rádio' });
  expect(
    screen.queryByRole('combobox', { name: 'Outra credencial do equipamento' }),
  ).not.toBeInTheDocument();
  expect(equipmentAPI.credentials).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(1);
});

function preparePreservedCredentialRefusal() {
  const { target } = prepareExplicitCredentialChoiceResponses(
    prepareExplicitCredentialChoiceTarget(),
  );
  target.role = RadioRoles.cpe;
  const stale = {
    ...reading,
    perspective: 'CPE',
    compatible: false,
    clients: [],
  };
  vi.mocked(api.current).mockResolvedValue(response({ reading: stale }));
  vi.mocked(api.status).mockResolvedValue(
    response({
      reading: stale,
      ap_code: RadioCodes.REMOTE_AUTH_FAILED,
      job: { id: 'radiojob', state: 'FAILED', code: 'PARTIAL' },
    }),
  );
  return target;
}
