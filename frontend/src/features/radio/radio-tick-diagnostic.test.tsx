import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { Reading } from './radio-reading';
import { reading } from '../../shared/testing/radio-test-fixtures';
it('tick OK com métrica inválida mantém o dado e apresenta o motivo legível com código só nos detalhes', () => {
  const view = render(
    <Reading
      reading={{
        ...reading,
        failure: { reason: 'tick-metric:tx-rate', bytes: 100 },
      }}
      title="AP"
      code="OK"
    />,
  );
  expect(screen.getByRole('status')).toHaveTextContent(
    'Uma métrica não foi informada neste ciclo; as demais foram preservadas.',
  );
  expect(
    screen.getByText('Detalhe técnico').closest('details'),
  ).toHaveTextContent('tick-metric:tx-rate');
  const outside = Array.from(view.container.querySelectorAll('p'))
    .filter((p) => !p.closest('details'))
    .map((p) => p.textContent)
    .join(' ');
  expect(outside).not.toContain('tick-metric:');
  expect(screen.getByText('CPE-I1')).toBeInTheDocument();
});
