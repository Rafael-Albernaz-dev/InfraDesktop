// Isolated test entry. Never imported by the production entrypoint.
import { createRoot } from 'react-dom/client';
import { useCallback, useState } from 'react';
import { Desktop } from '../src/desktop';
import { Equipment } from '../src/equipment';
import type { Device, EquipmentResult } from '../src/equipment-api';
import fixture from './fixture.json';
import '../src/style.css';
const query = new URLSearchParams(location.search),
  mode = query.get('state') || 'list';
const observation = {
  identity: 'Fixture router',
  version: '7.26 (stable)',
  family: 'v7',
  board: 'Test board',
  uptime: '1d2h',
  cpu_percent: 12,
  free_memory_bytes: 1024,
  total_memory_bytes: 2048,
  interfaces: [
    { name: 'ether1', kind: 'ether', running: true, disabled: false },
  ],
  observed_at: '2026-10-24T14:00:00Z',
  source: 'SSH · 127.0.0.1 · isolated visual fixture',
  quality: 'complete',
};
const devices = fixture.devices.map((d, i) => ({
  id: `00000000-0000-4000-8000-${String(i).padStart(12, '0')}`,
  name: d.name,
  address: d.ip,
  port: 22,
  transport: 'ssh',
  credential_id: 'read',
  revision: 2,
  active: mode !== 'inactive-only',
  fingerprint: mode === 'result' ? 'SHA256:TEST-FIXTURE-ONLY' : '',
  trust_origin: mode === 'result' ? 'Isolated fixture channel' : '',
  result: mode === 'result' ? observation : undefined,
})) as Device[];
const metadata = {
  id: 'read',
  alias: 'Consulta local de teste',
  user: 'reader',
  revision: 1,
  links: devices.length,
};
const result = (extra = {}) =>
  ({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: mode !== 'vault',
      total_devices: mode === 'empty' ? 0 : devices.length,
      ...extra,
    },
  }) as EquipmentResult;
const app = {
  ListEquipment: async (
    _p: string,
    _a: string,
    search: string,
    _cursor: number,
    inactive: boolean,
  ) =>
    result({
      devices:
        mode === 'empty'
          ? []
          : devices
              .filter((d) => d.active !== inactive)
              .filter((d) =>
                (d.name + ' ' + d.address)
                  .toLowerCase()
                  .includes(search.toLowerCase()),
              ),
    }),
  ListCredentials: async () => result({ credentials: [metadata] }),
  GetEquipment: async () => result({ device: devices[0] }),
  InspectTrust: async () =>
    result({
      job: {
        id: 'job',
        device_id: devices[0].id,
        kind: 'trust.inspect',
        state: 'SUCCEEDED',
        code: 'OK',
        fingerprint: 'SHA256:TEST-FIXTURE-ONLY',
      },
    }),
};
Object.assign(window, { go: { presentation: { App: app } } });
export function Scene() {
  const [create, setCreate] = useState(0);
  const [total, setTotal] = useState(0);
  const reportTotal = useCallback(
    (_context: string, count: number) => setTotal(count),
    [],
  );
  return (
    <Desktop
      page={4}
      inventoryHasDevices={total > 0}
      navigate={() => {}}
      brand={{ name: 'SerraLink Telecom', accent: 'teal', logo: '' }}
      account="Operador"
      theme={query.get('theme') || 'light'}
      setTheme={() => {}}
      connected
      busy={false}
      companies={[]}
      activeProfile="X"
      create={() => {}}
      enter={() => {}}
      logout={() => {}}
      unavailable={() => {}}
      addEquipment={() => setCreate((n) => n + 1)}
      inventory={
        <Equipment
          profile="X"
          activation="a"
          connected
          createRequest={create}
          primaryInHeader
          onTotal={reportTotal}
        />
      }
    />
  );
}
createRoot(document.getElementById('root')!).render(<Scene />);
