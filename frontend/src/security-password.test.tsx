import { render, screen } from '@testing-library/react';
import { it, expect, vi } from 'vitest';
import { Security } from './security';
import { message } from './microcopy';

it('shows incorrect password in the error variant and no theme controls', () => {
  const error = message('UNAUTHENTICATED', 'renew', 'PASSWORD_INCORRECT');
  render(
    <Security
      recovery=""
      busy={false}
      error={error}
      uncertain={false}
      lock={vi.fn()}
      renew={vi.fn()}
      acknowledge={vi.fn()}
    />,
  );
  expect(screen.getByRole('alert')).toHaveTextContent('SENHA INCORRETA');
  expect(screen.getByRole('alert')).toHaveClass('inline-error-danger');
  expect(
    screen.queryByRole('button', { name: 'Claro' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Escuro' }),
  ).not.toBeInTheDocument();
});
