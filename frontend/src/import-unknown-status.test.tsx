import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { ImportPreview } from './add-import-preview';

it('displays an unknown generated status and blocks confirmation', () => {
  const state = {
    filename: 'fixture.csv', busy: false, error: '', selected: [], setSelected: vi.fn(),
    confirm: vi.fn(), update: vi.fn(), review: vi.fn(),
    items: [
      { status: 'ready', row: { line: 1, name: 'Fixture A', address: '192.0.2.1', port: '22', role: 'other', block: '', credential: '' } },
      { status: 'unrecognized', row: { line: 2, name: 'Fixture B', address: '192.0.2.2',
        port: '22', role: 'other', block: '', credential: '' } },
    ],
  };
  render(<ImportPreview state={state as never} credentials={[]} cancel={vi.fn()} replace={vi.fn()} />);
  expect(screen.getByText('Situação desconhecida')).toBeInTheDocument();
  expect(screen.getByText('Resolva 1 linha para continuar.')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Cadastrar 1 equipamentos' })).toBeDisabled();
  expect(state.confirm).not.toHaveBeenCalled();
});
