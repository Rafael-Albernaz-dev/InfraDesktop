import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { PPPoESessionsTable } from './pppoe-sessions-view';
import { SessionAuthContent } from './pppoe-session-auth-content';
import { SessionLoading } from './pppoe-session-loading';
import type { PPRow } from '../../platform/wails/presentation';
import type { PPContext } from '../../platform/wails/pppoe-types';
import type { Device } from '../equipment/equipment-api';

const context: PPContext = {
  association: 'associated',
  secret_state: 'disabled',
  profile: 'SGP-PLANO-ID3',
  upload_limit: '1000000',
  download_limit: '1000000',
  profile_upload_limit: '20000000',
  profile_download_limit: '100000000',
};
const row: PPRow = {
  row_id: 'r',
  remote_id: '*1',
  session_id: 's',
  login: 'synthetic',
  address: '192.0.2.1',
  caller_id: '02:00:00:00:00:01',
  duplicates: 0,
  uptime: { known: true, min: 0, max: 0 },
  context,
};
it('always shows the disabled SGP plan with applied queue and warns only when both rates are known', () => {
  const view = render(<PPPoESessionsTable rows={[row]} onDetail={() => {}} />);
  expect(screen.getByText('SGP-PLANO-ID3')).toBeVisible();
  expect(screen.getByText('1 / 1 Mbps · diferente do plano')).toHaveClass(
    'diverge',
  );
  view.rerender(
    <PPPoESessionsTable
      rows={[
        { ...row, context: { ...context, profile_upload_limit: undefined } },
      ]}
      onDetail={() => {}}
    />,
  );
  expect(screen.getByText('1 / 1 Mbps')).toBeVisible();
  expect(screen.queryByText(/diferente do plano/)).not.toBeInTheDocument();
  view.rerender(
    <PPPoESessionsTable
      rows={[
        {
          ...row,
          context: {
            ...context,
            profile_upload_limit: '1000000',
            profile_download_limit: '1000000',
          },
        },
      ]}
      onDetail={() => {}}
    />,
  );
  expect(screen.queryByText(/diferente do plano/)).not.toBeInTheDocument();
});
it('identifies the SGP plan and applied speed in session information', () => {
  render(
    <SessionAuthContent
      selected={row}
      current={{
        observation_id: 'o',
        count: 1,
        provenance: { ended_at: '' },
        quality: { status: 'complete' },
      }}
      profile="p"
      activation="a"
      device={{ id: 'd' } as Device}
    />,
  );
  expect(
    screen
      .getAllByText('Plano (cadastro do SGP)')
      .find((element) => !element.closest('details')),
  ).toBeVisible();
  expect(
    screen
      .getAllByText('SGP-PLANO-ID3')
      .find((element) => !element.closest('details')),
  ).toBeInTheDocument();
  expect(
    screen
      .getAllByText('Velocidade aplicada agora')
      .find((element) => !element.closest('details')),
  ).toBeVisible();
});
it('shows the loading spinner and skeleton blocks while waiting for the first session data', () => {
  const view = render(
    <SessionLoading login="synthetic" device="CCR" back={() => {}} />,
  );
  expect(screen.getByText('Carregando sessão…')).toHaveClass('spin');
  expect(view.container.querySelectorAll('.skel')).toHaveLength(6);
  expect(screen.getAllByRole('tab')).toHaveLength(4);
  view.rerender(
    <SessionLoading
      login="synthetic"
      device="CCR"
      error="Leitura mudou"
      back={() => {}}
    />,
  );
  expect(screen.getByRole('alert')).toHaveTextContent('Leitura mudou');
  expect(
    screen.queryByRole('heading', { name: 'synthetic' }),
  ).not.toBeInTheDocument();
});
