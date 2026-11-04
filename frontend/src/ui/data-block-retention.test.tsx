import { useEffect } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { DataBlock } from './DataBlock';
it('mantém o mesmo conteúdo, edição e foco ao atualizar, falhar e recuperar', () => {
  const mounted = vi.fn();
  function Snapshot() {
    useEffect(() => {
      mounted();
    }, []);
    return <input aria-label="Dado salvo" defaultValue="salvo" />;
  }
  const snapshot = <Snapshot />;
  const { rerender } = render(
    <DataBlock
      title="Dado"
      source="SSH"
      data={{ state: 'ready', content: snapshot }}
    />,
  );
  const input = screen.getByRole('textbox', { name: 'Dado salvo' });
  fireEvent.change(input, { target: { value: 'edição preservada' } });
  input.focus();
  rerender(
    <DataBlock
      title="Dado"
      source="SSH"
      data={{ state: 'refreshing', content: snapshot }}
    />,
  );
  expect(screen.getByRole('textbox', { name: 'Dado salvo' })).toBe(input);
  expect(input).toHaveValue('edição preservada');
  expect(input).toHaveFocus();
  rerender(
    <DataBlock
      title="Dado"
      source="SSH"
      data={{
        state: 'failed',
        previous: snapshot,
        reason: 'Falhou',
        technical: 'synthetic',
        retry: vi.fn(),
      }}
    />,
  );
  expect(screen.getByRole('textbox', { name: 'Dado salvo' })).toBe(input);
  expect(input).toHaveFocus();
  rerender(
    <DataBlock
      title="Dado"
      source="SSH"
      data={{ state: 'ready', content: snapshot }}
    />,
  );
  expect(mounted).toHaveBeenCalledOnce();
  expect(input).toHaveFocus();
});
