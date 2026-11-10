import { RadioFamilies } from '../../features/radio/radio-contract';
import { RadioCodes } from '../../features/radio/radio-contract';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioLinkPanel } from '../../features/radio/radio';
import { radioAPI as api } from '../../features/radio/radio-api';
import {
  device,
  response,
  reading,
  link,
} from '../../features/radio/radio-test-fixtures';
import { equipmentAPI } from '../../features/equipment/equipment-api';

vi.mock('../../features/radio/radio-api', () => ({
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

vi.mock('../../features/equipment/equipment-api', () => ({
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
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      total_devices: 0,
      vault_available: true,
    },
  });
});

it('suggestion needs confirmation before link query; AP result survives an independent CPE failure', async () => {
  vi.mocked(api.link).mockResolvedValue(response({ suggestions: [link] }));
  vi.mocked(api.confirmLink).mockResolvedValue(response({ link }));
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      link,
      reading,
      ap_code: 'OK',
      cpe_code: RadioCodes.REMOTE_AUTH_FAILED,
      job: { id: 'job', state: 'FAILED', code: 'PARTIAL' },
    }),
  );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="2 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: /Confirmar sugestão/ }),
  );
  await screen.findByRole('button', { name: 'Atualizar' });
  expect(api.start).not.toHaveBeenCalled();
  expect(api.confirmLink).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar' }));
  await screen.findByText('-70 dBm');
  expect(screen.getByText('REMOTE_AUTH_FAILED')).toBeInTheDocument();
  expect(
    within(screen.getByRole('region', { name: 'Enlace rádio' })).getByText(
      'Uptime PPPoE: 2 h',
    ),
  ).toBeInTheDocument();
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeDisabled();
});

it('a link read error never claims a missing link; local reopening does not send SSH', async () => {
  vi.mocked(api.link)
    .mockResolvedValueOnce({ ...response(), code: 'SERVICE_UNAVAILABLE' })
    .mockResolvedValueOnce(response({ link }));
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reabrir vínculo armazenado' }),
  );
  expect(
    screen.queryByText('Nenhum vínculo confirmado para esta sessão.'),
  ).not.toBeInTheDocument();
  await screen.findByRole('button', { name: 'Atualizar' });
  expect(api.start).not.toHaveBeenCalled();
  expect(api.link).toHaveBeenCalledTimes(2);
});

it('one-click proposal identifies AP and credential, then clears on an ambiguous refresh', async () => {
  prepareAmbiguousProposalRefresh();
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="2 h"
      blocked={false}
    />,
  );
  const confirm = await screen.findByRole('button', {
    name: 'Confirmar enlace sugerido',
  });
  expect(confirm).toHaveClass('primary');
  const panel = screen.getByRole('region', { name: 'Enlace rádio' });
  expect(panel.querySelectorAll('button.primary')).toHaveLength(1);
  expect(panel).toHaveTextContent('AP sintético P04 (192.0.2.10)');
  expect(panel).toHaveTextContent('Credencial CPE escolhida');
  expect(api.confirmLink).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar índice' }));
  await screen.findByText(
    'MAC/IP visto em mais de um registro ou AP. Use o vínculo manual.',
  );
  expect(
    screen.queryByRole('button', { name: 'Confirmar enlace sugerido' }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Vincular CPE e AP' }),
  ).toBeEnabled();
  expect(api.start).toHaveBeenCalledTimes(1);
});

it('automatically refetches link via api.link after confirming CPE editing without sending new SSH', async () => {
  vi.mocked(api.link).mockResolvedValue(response({ link }));
  vi.mocked(api.confirmLink).mockResolvedValue(
    response({ link: { ...link, mac: '00:11:22:33:44:55' } }),
  );
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="2 h"
      blocked={false}
    />,
  );
  const editButton = await screen.findByRole('button', {
    name: 'Revisar vínculo',
  });
  fireEvent.click(editButton);
  const form = (
    await screen.findByRole('button', {
      name: 'Confirmar vínculo',
    })
  ).closest('form')!;
  fireEvent.submit(form);
  await waitFor(() => expect(api.confirmLink).toHaveBeenCalledTimes(1));
  expect(api.link).toHaveBeenCalledTimes(2);
  expect(api.start).not.toHaveBeenCalled();
});

function prepareAmbiguousProposalRefresh() {
  const index = {
    observed_at: new Date().toISOString(),
    complete: true,
    targets: { ap: 'OK' },
  };
  const proposal = {
    choice: link,
    address: '192.0.2.20',
    family: RadioFamilies.automatic,
    credential_id: 'selected',
    credential_alias: 'CPE escolhida',
    ap_name: 'AP sintético P04',
    ap_address: '192.0.2.10',
  };
  vi.mocked(api.link).mockResolvedValue(
    response({
      index,
      proposal,
      match_reason: 'Match inequívoco por MAC/IP na registration-table.',
    }),
  );
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'idx', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      index,
      match_reason:
        'MAC/IP visto em mais de um registro ou AP. Use o vínculo manual.',
      job: { id: 'idx', state: 'SUCCEEDED', code: 'OK' },
    }),
  );
  return { index, proposal };
}
