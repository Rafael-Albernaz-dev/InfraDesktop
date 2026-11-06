import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { SessionLinkSummary } from './session-link-summary';

it('shows saved AP and CPE readings without inventing missing fields', () => {
  const open = vi.fn();
  render(
    <SessionLinkSummary
      link={{ ap_id: 'ap-fixture', cpe_id: 'cpe-fixture' } as never}
      names={{ 'ap-fixture': 'AP sintético', 'cpe-fixture': 'CPE sintética' }}
      reading={
        {
          compatible: true,
          metrics: [
            { name: 'signal', value: -58, unit: 'dBm', quality: 'observed' },
            { name: 'ccq', value: 92, unit: '%', quality: 'observed' },
            { name: 'rate_tx', value: 117, unit: 'Mbps', quality: 'observed' },
            { name: 'rate_rx', value: 130, unit: 'Mbps', quality: 'observed' },
          ],
        } as never
      }
      open={open}
    />,
  );
  expect(screen.getByText('-58 dBm')).toBeVisible();
  expect(screen.getByText('92%')).toBeVisible();
  expect(screen.getByText('117/130 Mbps')).toBeVisible();
  screen.getByText('AP sintético').click();
  expect(open).toHaveBeenCalledWith('ap-fixture');
});
it('marks absent or incompatible radio readings as not informed', () => {
  render(
    <SessionLinkSummary
      link={{ ap_id: 'ap-fixture', cpe_id: 'cpe-fixture' } as never}
      names={{}}
      reading={undefined}
    />,
  );
  expect(screen.getAllByText('Não informado nesta leitura')).toHaveLength(5);
});
