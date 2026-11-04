import { render, screen, fireEvent } from '@testing-library/react';
import { it, expect, vi } from 'vitest';
import { Appearance } from './appearance';
const access = { saveBrand: vi.fn(), setTheme: vi.fn() };
it('keeps retained branding data editor isolated and cancellation restores its draft', async () => {
  render(
    <Appearance
      brand={{ name: '', logo: '', accent: 'teal' }}
      theme="light"
      setTheme={access.setTheme}
      companySelected
      ready
      saveBrand={access.saveBrand}
      busy={false}
    />,
  );
  const visual = screen.getByLabelText('Nome visual', { exact: true });
  fireEvent.change(visual, { target: { value: 'Draft only' } });
  fireEvent.click(screen.getByRole('button', { name: 'Violeta' }));
  expect(screen.getByRole('button', { name: 'Salvar' })).toBeEnabled();
  expect(access.saveBrand).not.toHaveBeenCalled();
  expect(screen.queryByText('412')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
  expect(visual).toHaveValue('');
  expect(screen.getByRole('button', { name: 'Turquesa' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  expect(access.setTheme).not.toHaveBeenCalled();
});
