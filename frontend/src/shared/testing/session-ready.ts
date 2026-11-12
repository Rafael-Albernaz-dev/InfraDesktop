import { screen } from '@testing-library/react';

// The session route is visible before its first detail response.
export function waitForSessionReady() {
  return screen.findByRole('region', {
    name: 'Detalhes da sessão',
    busy: false,
  });
}
