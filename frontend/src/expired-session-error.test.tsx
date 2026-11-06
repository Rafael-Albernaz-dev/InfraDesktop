import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Security } from './security';
import { message } from './microcopy';

it('does not describe an expired recovery session as an incorrect password', () => {
  expect(message('UNAUTHENTICATED', 'renew')).toBe(
    'Entre novamente para continuar.',
  );
});

it('shows the expired session message in the recovery modal', () => {
  render(
    <Security
      errorKind="general"
      recovery=""
      busy={false}
      error={message('UNAUTHENTICATED', 'renew')}
      uncertain={false}
      lock={vi.fn()}
      renew={vi.fn()}
      acknowledge={vi.fn()}
    />,
  );
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Entre novamente para continuar.',
  );
  expect(screen.getByRole('alert')).not.toHaveClass('inline-error-danger');
});
