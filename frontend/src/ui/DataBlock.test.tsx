import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { DataBlock } from './DataBlock';
import type { DataState } from './DataBlock';

describe('bloco de dado com um único estado', () => {
  const action = vi.fn();
  const cases: [DataState, string][] = [
    [{ state: 'unread', action }, 'Ainda sem leitura'],
    [{ state: 'loading' }, 'Carregando'],
    [
      { state: 'refreshing', content: 'Leitura anterior 42' },
      'Leitura anterior 42',
    ],
    [{ state: 'ready', content: 'Leitura válida 42' }, 'Leitura válida 42'],
    [
      { state: 'empty', message: 'Nenhum cliente encontrado' },
      'Nenhum cliente encontrado',
    ],
    [
      {
        state: 'failed',
        reason: 'Tempo esgotado',
        technical: 'TIMEOUT',
        retry: action,
      },
      'Tempo esgotado',
    ],
    [
      { state: 'not-supported', resource: 'potência por taxa' },
      'Este modelo não informa potência por taxa',
    ],
    [{ state: 'unbuilt', slice: 'L-BASE-2' }, 'Ainda não existe (L-BASE-2)'],
  ];
  it.each(cases)('apresenta $state sem apagar dado válido', (data, text) => {
    const { container } = render(
      <DataBlock title="Rádio" source="SSH" age="há 12 s" data={data} />,
    );
    if (data.state === 'loading')
      expect(screen.getByRole('status')).toHaveAccessibleName(text);
    else expect(screen.getByText(text)).toBeVisible();
    expect(container.querySelectorAll('[data-state]')).toHaveLength(1);
    if (data.state === 'refreshing')
      expect(screen.getByRole('status')).toHaveTextContent('atualizando');
    if (data.state === 'unread' || data.state === 'loading')
      expect(screen.queryByText(/há 12 s/)).toBeNull();
    else expect(screen.getByText(/há 12 s/)).toBeVisible();
  });
  it('encaminha a recuperação explícita e conserva o motivo técnico', () => {
    render(
      <DataBlock
        title="Rádio"
        source="SSH"
        data={{
          state: 'failed',
          reason: 'Falhou',
          technical: 'TIMEOUT',
          retry: action,
        }}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
    expect(action).toHaveBeenCalledOnce();
    expect(screen.getByText('Motivo: TIMEOUT')).toBeInTheDocument();
  });
});
