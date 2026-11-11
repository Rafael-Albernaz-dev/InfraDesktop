import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import {
  RadioFamilies,
  RadioRoles,
} from '../../features/equipment/radio-contract';
import { RadioEquipment } from '../../features/radio/radio';
import { radioAPI as api } from '../../features/equipment/radio-api';
import {
  device,
  response,
  reading,
} from '../../shared/testing/radio-test-fixtures';
vi.mock('../../features/equipment/radio-api', async () =>
  (await import('../../shared/testing/radio-api-mock')).radioAPIMock({}),
);
vi.mock('../../features/equipment/equipment-api', async () =>
  (await import('../../shared/testing/equipment-api-mock')).equipmentAPIMock({
    equipmentMessage: (c: string) => c,
  }),
);
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.link).mockResolvedValue(response({ link: undefined }));
});

it('keeps stored-radio recovery reachable when a failed refresh leaves an unsupported reading', async () => {
  const target = { ...device, role: RadioRoles.cpe };
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading: { ...reading, family: RadioFamilies.unsupported },
    }),
  );
  vi.mocked(api.start).mockResolvedValue({
    ...response(),
    code: 'CONNECT_FAILED',
  });
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={target}
      blocked={false}
      tab="Enlace"
    />,
  );
  await screen.findByText(/Driver ausente/);
  expect(
    screen.queryByRole('button', { name: 'Reabrir rádio armazenado' }),
  ).not.toBeInTheDocument();
  fireEvent.click(await screen.findByRole('button', { name: 'Atualizar' }));
  expect(
    await screen.findByRole('button', { name: 'Reabrir rádio armazenado' }),
  ).toBeInTheDocument();
  expect(api.current).toHaveBeenCalledTimes(1);
});
