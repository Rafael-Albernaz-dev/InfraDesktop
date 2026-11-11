import { act, fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioEquipment } from '../../features/radio/radio';
import { radioAPI as api } from '../../features/equipment/radio-api';
import type {
  RadioReading,
  RadioResult,
} from '../../features/equipment/radio-api';
import { equipmentAPI } from '../../features/equipment/equipment-api';
import {
  device,
  reading,
  response,
} from '../../shared/testing/radio-test-fixtures';
vi.mock('../../features/equipment/radio-api', async () =>
  (await import('../../shared/testing/radio-api-mock')).radioAPIMock({}),
);
vi.mock('../../features/equipment/equipment-api', async () =>
  (await import('../../shared/testing/equipment-api-mock')).equipmentAPIMock({
    equipmentMessage: (code: string) => code,
  }),
);
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(equipmentAPI.get).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      device,
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
    },
  });
});
function deferredPage() {
  let resolve!: (value: RadioResult) => void,
    reject!: (reason: unknown) => void;
  const promise = new Promise<RadioResult>((accept, fail) => {
    resolve = accept;
    reject = fail;
  });
  return { promise, resolve, reject };
}
function clientReading(id: string, name: string): RadioReading {
  return {
    ...reading,
    id,
    clients: reading.clients.map((client) => ({ ...client, name })),
  };
}
function configurePageRace(pending: ReturnType<typeof deferredPage>) {
  vi.mocked(api.current)
    .mockResolvedValueOnce(
      response({ reading, total: 16, more: true, next_offset: 8 }),
    )
    .mockImplementationOnce(() => pending.promise);
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'job-B', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      reading: clientReading('reading-B', 'Cliente B'),
      job: { id: 'job-B', state: 'SUCCEEDED', code: 'OK' },
    }),
  );
}
async function finishOldPage(
  pending: ReturnType<typeof deferredPage>,
  kind: string,
) {
  await act(async () => {
    if (kind === 'rejection') pending.reject(new Error('page failed'));
    else if (kind === 'error')
      pending.resolve({ ...response(), code: 'UNAVAILABLE' });
    else pending.resolve(response({ reading, total: 16, more: false }));
    await pending.promise.catch(() => undefined);
  });
}
it.each(['success', 'error', 'rejection'])(
  'preserves newer query B and offset when an older page finishes with %s',
  async (kind) => {
    const pending = deferredPage();
    configurePageRace(pending);
    render(
      <RadioEquipment
        tab="Clientes"
        profile="company"
        activation="active"
        device={device}
        blocked={false}
      />,
    );
    await screen.findByRole('cell', { name: /CPE-I1/ });
    fireEvent.click(screen.getByRole('button', { name: 'Próxima' }));
    expect(api.current).toHaveBeenCalledTimes(2);
    fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
    await screen.findByRole('cell', { name: /Cliente B/ });
    await finishOldPage(pending, kind);
    expect(screen.getByRole('cell', { name: /Cliente B/ })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Clientes anteriores' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('UNAVAILABLE')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Página indisponível. Reabra a leitura salva.'),
    ).not.toBeInTheDocument();
    expect(api.current).toHaveBeenCalledTimes(2);
    expect(api.start).toHaveBeenCalledTimes(1);
  },
);
it('accepts a legitimate page when no newer query owns the view', async () => {
  vi.mocked(api.current)
    .mockResolvedValueOnce(
      response({ reading, total: 16, more: true, next_offset: 8 }),
    )
    .mockResolvedValueOnce(
      response({
        reading: clientReading('page-A', 'Página legítima'),
        total: 16,
      }),
    );
  render(
    <RadioEquipment
      tab="Clientes"
      profile="company"
      activation="active"
      device={device}
      blocked={false}
    />,
  );
  await screen.findByRole('cell', { name: /CPE-I1/ });
  fireEvent.click(screen.getByRole('button', { name: 'Próxima' }));
  await screen.findByRole('cell', { name: /Página legítima/ });
  expect(
    screen.getByRole('button', { name: 'Clientes anteriores' }),
  ).toBeInTheDocument();
  expect(api.current).toHaveBeenCalledTimes(2);
  expect(api.start).not.toHaveBeenCalled();
});
