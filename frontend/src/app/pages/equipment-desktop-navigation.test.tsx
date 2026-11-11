import { device, data } from '../../shared/testing/equipment-test-fixture';

import { openDeviceSheet } from '../test-navigation';
import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';
import { Desktop, Statusbar } from '../desktop';
import { equipmentAPI as api } from '../../features/equipment/equipment-api';

vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({
    deviceMonitoringAPI: {
      current: vi.fn().mockResolvedValue({
        code: 'OK',
        profile_id: 'company',
        activation_id: 'active',
        data: { total: 0 },
      }),
    },
  }),
);
beforeEach(resetEquipmentMocks);
import { resetEquipmentMocks } from '../../shared/testing/equipment-setup';
it('sidebar routes remain keyboard accessible and profile Escape restores its trigger', () => {
  const navigate = vi.fn();
  render(
    <Desktop
      page={3}
      navigate={navigate}
      brand={{ name: 'X', accent: 'teal', logo: '' }}
      account="operator"
      theme="light"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="X"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
    />,
  );
  const equipment = screen.getByRole('link', { name: 'Equipamentos' });
  fireEvent.click(equipment);
  expect(navigate).toHaveBeenCalledWith('equipment');
  fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
  expect(navigate).toHaveBeenCalledWith('settings');
  const trigger = screen.getByRole('button', { name: /Conta:/ });
  fireEvent.keyDown(trigger, { key: 'ArrowDown' });
  expect(screen.getByRole('menuitem', { name: 'Tema escuro' })).toHaveFocus();
  fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Tema escuro' }), {
    key: 'End',
  });
  expect(screen.getByRole('menuitem', { name: 'Sair da conta' })).toHaveFocus();
  fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Sair da conta' }), {
    key: 'Escape',
  });
  expect(trigger).toHaveFocus();
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
});
it('service status and last valid data remain truthful after a reading, failed refresh, navigation and context change', async () => {
  const observation = {
    identity: 'Last valid reading',
    version: '7',
    family: 'v7',
    board: 'B',
    uptime: '1h',
    cpu_percent: 1,
    free_memory_bytes: 1,
    total_memory_bytes: 2,
    interfaces: [],
    observed_at: '2026-10-24T14:00:00Z',
    source: 'SSH',
    quality: 'complete',
  };
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, result: observation }]),
  );
  vi.mocked(api.get).mockResolvedValue({
    ...data(),
    data: { ...data().data!, device: { ...device, result: observation } },
  });
  const view = render(
    <>
      <Statusbar connected />
      <Equipment key="X" profile="X-footer" activation="footer" connected />
    </>,
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('tab', { name: 'Resumo' }));
  expect(await screen.findByText('Last valid reading')).toBeVisible();
  expect(screen.getByText('Serviço ativo')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  vi.mocked(api.list).mockResolvedValue({
    ...data(),
    code: 'SERVICE_UNAVAILABLE',
  });
  fireEvent.change(screen.getByLabelText('Buscar'), {
    target: { value: 'refresh' },
  });
  await screen.findByText(/O serviço está indisponível/);
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('tab', { name: 'Resumo' }));
  expect(await screen.findByText('Last valid reading')).toBeVisible();
  view.rerender(<Statusbar connected />);
  expect(screen.queryByText('Sem coletas')).not.toBeInTheDocument();
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(
    <>
      <Statusbar connected />
      <Equipment key="Y" profile="Y-footer" activation="other" connected />
    </>,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.queryByText('Rede não consultada')).not.toBeInTheDocument();
  expect(screen.getByText('Serviço ativo')).toBeInTheDocument();
});
it('the full desktop owns only one Add action in empty and populated inventory states', async () => {
  const view = render(
    <Desktop
      page={4}
      navigate={() => {}}
      brand={{ name: '', accent: 'teal', logo: '' }}
      account="Fixture"
      theme="light"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="X"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
      inventory={<Equipment profile="cta" activation="a" connected />}
    />,
  );
  await screen.findByRole('cell', { name: 'Router X' });
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(
    <Desktop
      page={4}
      navigate={() => {}}
      brand={{ name: '', accent: 'teal', logo: '' }}
      account="Fixture"
      theme="dark"
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="Y"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
      inventory={
        <Equipment key="empty" profile="cta-empty" activation="b" connected />
      }
    />,
  );
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
});
