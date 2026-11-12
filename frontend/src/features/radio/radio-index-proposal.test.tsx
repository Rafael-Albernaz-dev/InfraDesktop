import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { RadioFamilies } from '../equipment/radio-contract';
import { RadioIndexTable, RadioLinkMatchReason } from './radio-link-index';
import { RadioLinkProposal } from './radio-link-association';
import { useRadioLinkState } from './radio-link-state';
import { radioAPI } from '../equipment/radio-api';
import { device, response } from '../../shared/testing/radio-test-fixtures';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
const warning =
  '1 AP não respondeu: AP-SUL (192.0.2.21), CONNECT_FAILED. A sugestão vem dos APs que responderam';
function IndexProposal() {
  const state = useRadioLinkState({
    profile: 'company',
    activation: 'active',
    device,
    observation: 'o',
    row: 'r',
    pppoeUptime: '',
    blocked: false,
  });
  return (
    <>
      <RadioLinkMatchReason state={state} />
      <RadioLinkProposal state={state} />
      <RadioIndexTable state={state} />
    </>
  );
}
it('shows the failed AP warning before the enabled confirmation and lists the reading of every AP', async () => {
  const at = new Date(Date.now() - 8 * 60000).toISOString();
  vi.mocked(radioAPI.link).mockResolvedValue(
    response({
      match_reason: warning,
      proposal: {
        ap_name: 'AP-NORTE',
        ap_address: '192.0.2.20',
        address: '192.0.2.50',
        family: RadioFamilies.automatic,
        credential_id: 'c',
        credential_alias: 'CPE',
        choice: {
          ap_id: 'north',
          cpe_id: '',
          mac: '02:00:00:00:00:01',
          interface: 'wlan1',
          bssid: '02:00:00:00:00:02',
          evidence: 'registration-table',
          origin: 'sugerido-confirmado',
        },
      },
      index: {
        observed_at: at,
        complete: false,
        targets: { north: 'OK', south: 'CONNECT_FAILED' },
        labels: {
          north: 'AP-NORTE (192.0.2.20)',
          south: 'AP-SUL (192.0.2.21)',
        },
      },
    }),
  );
  render(<IndexProposal />);
  const notice = await screen.findByText(warning);
  const confirm = screen.getByRole('button', {
    name: 'Confirmar enlace sugerido',
  });
  expect(notice).toHaveClass('diverge');
  expect(notice.compareDocumentPosition(confirm) & 4).toBe(4);
  expect(confirm).toBeEnabled();
  expect(screen.getByRole('columnheader', { name: 'Leitura' })).toBeVisible();
  expect(screen.getByText('há 8 min', { selector: 'td' })).toBeVisible();
  expect(screen.getByText(/Não foi possível alcançar a porta SSH/)).toHaveClass(
    'diverge',
  );
});
