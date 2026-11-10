import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Security } from './security';
const base = {
  recovery: '',
  busy: false,
  uncertain: false,
  lock: vi.fn(),
  renew: vi.fn(),
  acknowledge: vi.fn(),
};
it('uses the explicit error state rather than the displayed words', () => {
  const view = render(
    <Security {...base} error="SENHA INCORRETA" errorKind="general" />,
  );
  expect(screen.getByRole('alert')).not.toHaveClass('inline-error-danger');
  view.rerender(
    <Security
      {...base}
      error="Texto localizado da senha atual"
      errorKind="incorrect-password"
    />,
  );
  expect(screen.getByRole('alert')).toHaveClass('inline-error-danger');
});

import { message } from './microcopy';
import { accessErrorKind } from './access-error-state';
it('distinguishes administrator denial, cancellation and an unavailable dialog', () => {
  expect(message('ADMIN_AUTH_DENIED', 'reset')).toBe(
    'A autorização de administrador foi negada. Confira a senha de administrador e tente novamente.',
  );
  expect(message('ADMIN_AUTH_CANCELLED', 'reset')).toBe(
    'A autorização foi cancelada.',
  );
  expect(message('ADMIN_AUTH_UNAVAILABLE', 'reset')).toBe(
    'Não foi possível abrir a autorização de administrador neste computador.',
  );
  expect(accessErrorKind('UNAUTHENTICATED', 'renew')).toBe('general');
  expect(
    accessErrorKind('UNAUTHENTICATED', 'renew', 'PASSWORD_INCORRECT'),
  ).toBe('incorrect-password');
});
