import { fireEvent, screen } from '@testing-library/react';

// Follow the public list → sheet → Acesso journey used by operation regressions.
export async function openDeviceSheet(name: string) {
  let heading = screen.queryByRole('heading', { name });
  if (!heading) {
    const back = screen.queryByRole('button', { name: 'Voltar' });
    if (back && !screen.queryByRole('dialog')) fireEvent.click(back);
    const cell = await screen.findByRole('cell', {
      name: (label) =>
        label === name ||
        label.replace(/\s/g, '') === (name + '(inativo)').replace(/\s/g, ''),
    });
    if (screen.queryByRole('dialog')) return cell;
    fireEvent.click(cell);
    heading = await screen.findByRole('heading', { name });
  }
  const access = screen.queryByRole('tab', { name: 'Acesso' });
  if (access && access.getAttribute('aria-selected') !== 'true')
    fireEvent.click(access);
  return heading;
}

export async function beginAdd() {
  const back = screen.queryByRole('button', { name: 'Voltar' });
  if (!screen.queryByRole('button', { name: 'Adicionar equipamento' }) && back)
    fireEvent.click(back);
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar equipamento' }),
  );
}
