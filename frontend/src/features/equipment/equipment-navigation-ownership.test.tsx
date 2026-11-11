import { renderHook } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { equipmentAPI } from './equipment-api';
import { useEquipmentNavigator } from './equipment-navigator';
import type { Device, EquipmentResult } from './equipment-api';
vi.mock('./equipment-api', async () =>
  (await import('../../shared/testing/equipment-api-mock')).equipmentAPIMock({
    equipmentMessage: (code: string) => code,
  }),
);
it('resposta de navegação não atravessa ativação nem substitui uma escolha mais recente', async () => {
  let resolve!: (r: EquipmentResult) => void;
  vi.mocked(equipmentAPI.get).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  const refs = {
    scope: { current: 'company|active' },
    surface: { current: 0 },
    mounted: { current: true },
  };
  const open = vi.fn();
  const navigation = renderHook(() =>
    useEquipmentNavigator({
      profile: 'company',
      activation: 'active',
      scopeRef: refs.scope,
      surfaceRef: refs.surface,
      mountedRef: refs.mounted,
      open,
      error: vi.fn(),
    }),
  );
  const navigate = navigation.result.current;
  const response = {
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      device: { id: 'concentrator' } as Device,
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
    },
  };
  navigate('concentrator');
  refs.scope.current = 'company|new-activation';
  resolve(response);
  await Promise.resolve();
  expect(open).not.toHaveBeenCalled();
  refs.scope.current = 'company|active';
  navigate('concentrator');
  refs.surface.current++;
  resolve(response);
  await Promise.resolve();
  expect(open).not.toHaveBeenCalled();
  navigate('concentrator');
  resolve(response);
  await Promise.resolve();
  expect(open).toHaveBeenCalledOnce();
  navigate('concentrator');
  refs.mounted.current = false;
  resolve(response);
  await Promise.resolve();
  expect(open).toHaveBeenCalledOnce();
});
