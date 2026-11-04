import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Dialog } from './Dialog';
import { Field } from './Field';
import { readFileSync } from 'node:fs';
const css = readFileSync('src/ui/kit.css', 'utf8');

it('mantém erro legível e fechamento por teclado no modal do kit', () => {
  const close = vi.fn();
  render(
    <Dialog title="Adicionar equipamento" close={close}>
      <Field label="Nome" aria-invalid="true" aria-describedby="erro">
        <span id="erro" className="field-error" role="alert">
          Informe o nome
        </span>
      </Field>
    </Dialog>,
  );
  const dialog = screen.getByRole('dialog', { name: 'Adicionar equipamento' });
  expect(dialog).toHaveClass('dlg');
  expect(screen.getByRole('button', { name: 'Fechar painel' })).toHaveFocus();
  expect(screen.getByRole('textbox')).toHaveAccessibleDescription(
    'Informe o nome',
  );
  expect(screen.getByRole('alert')).toBeVisible();
  expect(css).toMatch(/\.field-error\s*\{/);
  expect(css).toMatch(/\.inline-error\s*\{/);
  expect(css).toMatch(/\.operation-error\s*\{/);
  expect(css).toMatch(/\.empty-state\s*\{/);
  fireEvent.keyDown(dialog, { key: 'Escape' });
  expect(close).toHaveBeenCalledOnce();
});
