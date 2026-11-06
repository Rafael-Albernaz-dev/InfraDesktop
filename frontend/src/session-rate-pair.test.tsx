import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { SessionRatePair } from './session-rate-pair';

it('uses one shared scale for exact rates and retains exact values', () => {
  render(
    <SessionRatePair
      download={{ min: 9800000, max: 9800000 }}
      upload={{ min: 900000, max: 900000 }}
    />,
  );
  expect(screen.getByText('9,8 / 0,9 Mbps')).toHaveAttribute(
    'title',
    '9.800.000 bit/s / 900.000 bit/s',
  );
});
it('keeps uncertainty and missing values explicit', () => {
  render(<SessionRatePair download={{ min: 1600, max: 1601 }} />);
  expect(screen.getByTitle('1.600–1.601 bit/s')).toBeVisible();
  expect(screen.getByText('Não informado nesta leitura')).toBeVisible();
});
