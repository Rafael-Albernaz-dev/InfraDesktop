import { vi } from 'vitest';
import { device, data } from './equipment-test-fixture';
import {
  equipmentAPI as api,
  deviceMonitoringAPI as monitoringAPI,
  pppoeReadAPI as readAPI,
} from '../../platform/wails/presentation';
import type { EquipmentResult } from '../../features/equipment/equipment-api';
export function resetEquipmentMocks() {
  vi.resetAllMocks();
  vi.mocked(monitoringAPI.current).mockImplementation(
    async (profile_id, activation_id) => ({
      code: 'OK',
      profile_id,
      activation_id,
      data: { total: 0 },
    }),
  );
  vi.mocked(readAPI.cancel).mockResolvedValue({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { total: 0 },
  });
  vi.mocked(readAPI.cancelOperation).mockResolvedValue({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { total: 0 },
  });
  vi.mocked(api.list).mockResolvedValue(data());
  vi.mocked(api.get).mockImplementation(async (_p, _a, id) => {
    const latest = (await vi.mocked(api.list).mock.results.at(-1)?.value) as
      EquipmentResult | undefined;
    const target = latest?.data?.devices.find((d) => d.id === id) || device;
    return { ...data(), data: { ...data().data!, device: target } };
  });
  vi.mocked(api.credentials).mockResolvedValue({
    ...data(),
    data: {
      ...data().data!,
      credentials: [
        {
          id: 'credential',
          alias: 'Consulta',
          user: 'reader',
          revision: 1,
          links: 2,
        },
      ],
    },
  } as EquipmentResult);
}
