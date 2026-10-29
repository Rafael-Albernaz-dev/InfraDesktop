// Isolated test entry. Never imported by the production entrypoint.
import { createRoot } from 'react-dom/client';
import { useCallback, useState } from 'react';
import { Desktop } from '../src/desktop';
import { Equipment } from '../src/equipment';
import { RadioLinkPanel } from '../src/radio';
import type { Device, EquipmentResult } from '../src/equipment-api';
import '../src/style.css';
const query = new URLSearchParams(location.search),
  mode = query.get('state') || 'list';
const devices = [
  {
    id: '00000000-0000-4000-8000-000000000001',
    name: 'AP sintético',
    role: 'ap',
    address: '192.0.2.10',
    port: 22,
    transport: 'ssh',
    credential_id: 'read',
    revision: 2,
    active: true,
    fingerprint: 'SHA256:SYNTHETIC-PIN',
    trust_origin: 'synthetic visual fixture',
  },
] as Device[];

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
let listAttempts = 0;
const app = {
  ListEquipment: async (
    _p: string,
    _a: string,
    search: string,
    _cursor: number,
    inactive: boolean,
  ) => {
    if (mode === 'inventory-error' && ++listAttempts === 1)
      throw new Error('isolated list failure');
    return result({
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
    });
  },
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
  CancelQuery: async () =>
    result({
      job: {
        id: 'job',
        device_id: devices[0].id,
        kind: 'trust.inspect',
        state: 'CANCELLED',
        code: 'CANCELLED',
      },
    }),
};

const at = '2026-10-29T18:00:00Z';
const metric = (
  name: string,
  value: number,
  unit: string,
  direction = 'CPE → AP',
) => ({
  name,
  value,
  unit,
  direction,
  perspective: 'AP',
  device_id: devices[0].id,
  interface: 'wlan1',
  observed_at: at,
  quality: 'observed',
});
const clients = Array.from({ length: mode === 'lotado' ? 22 : 6 }, (_, i) => ({
  mac: '02:00:00:00:00:' + String(i + 1).padStart(2, '0'),
  name: 'CPE sintética ' + (i + 1),
  interface: i === 5 ? 'virtual' : 'wlan1',
  bssid: i === 5 ? '02:00:00:00:01:02' : '02:00:00:00:01:01',
  wireless_uptime: '1d2h',
  last_activity: '90ms',
  metrics: [
    metric('signal', -61 - i, 'dBm'),
    { ...metric('chain0', -63 - i, 'dBm'), best: -59, worst: -72 },
    metric('snr', 31 - i, 'dB'),
    metric('ccq_tx', 98.8, '%', 'AP → CPE'),
    metric('rate_tx', 60, 'Mbps', 'AP → CPE'),
  ],
}));
const radioReading = {
  id: 'reading',
  device_id: devices[0].id,
  family: 'routeros-wireless-v6',
  version: '6.49.10',
  perspective: 'AP',
  observed_at: at,
  compatible: true,
  revision: 2,
  credential_revision: 1,
  connection: 'synthetic',
  had_clients: true,
  seconds: 1,
  interfaces: [
    {
      name: 'wlan1',
      bssid: '02:00:00:00:01:01',
      band: '5ghz-a/n',
      configured_width: '20/40mhz-eC',
      channel: '5215/20-eC/an',
      clients: mode === 'zero' ? 0 : mode === 'lotado' ? 22 : 5,
      had_clients: true,
      power_metrics: [
        {
          ...metric('power_HT40-7', 23, 'dBm', 'AP → CPE'),
          label: 'Potência · HT40-7',
        },
      ],
      metrics: [
        metric('frequency', 5215, 'MHz', 'configuration'),
        metric('noise', -107, 'dBm', 'receive'),
        metric('ccq', 100, '%', 'AP → CPE'),
      ],
    },
    {
      name: 'virtual',
      master: 'wlan1',
      bssid: '02:00:00:00:01:02',
      clients: mode === 'zero' || mode === 'lotado' ? 0 : 1,
      had_clients: mode !== 'zero',
      metrics: [],
    },
  ],
  clients: mode === 'zero' ? [] : clients.slice(0, 8),
  metrics: [],
};
const link = {
  concentrator_id: 'concentrator',
  scope: 'pppoe-active-v1',
  login: 'X',
  cpe_id: 'cpe',
  ap_id: devices[0].id,
  mac: clients[0].mac,
  interface: 'wlan1',
  bssid: '02:00:00:00:01:01',
  evidence: 'synthetic confirmed MAC',
  origin: 'manual',
  confirmed_at: at,
  confirmed_now: mode !== 'link-ap-fail',
};
const cpeReading = {
  ...radioReading,
  id: 'cpe-reading',
  device_id: 'cpe',
  family: 'intelbras-wom',
  perspective: 'CPE',
  ssh_legacy: ['ssh-rsa', 'diffie-hellman-group14-sha1'],
  interfaces: [],
  clients: [],
  wireless_uptime: undefined,
  ap_bssid: link.bssid,
  wireless_mac: link.mac,
  metrics: [
    {
      ...metric('signal', -55, 'dBm', 'AP → CPE'),
      perspective: 'CPE',
      device_id: 'cpe',
    },
    {
      ...metric('chain0', -58, 'dBm', 'AP → CPE'),
      perspective: 'CPE',
      device_id: 'cpe',
      best: -57,
      worst: -60,
    },
  ],
};
const radioResult = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'X',
  activation_id: 'a',
  data: {
    total: mode === 'zero' ? 0 : clients.length,
    more: mode === 'lotado',
    next_offset: 8,
    ...data,
  },
});
let starts = 0;
const radioApp = {
  GetRadio: async () =>
    mode === 'error'
      ? { ...radioResult(), code: 'CONNECT_FAILED' }
      : radioResult(
          mode === 'initial'
            ? {}
            : {
                reading: radioReading,
                profile: {
                  device_id: devices[0].id,
                  family: radioReading.family,
                  confirmed: true,
                  warning: 15,
                  critical: 20,
                },
              },
        ),
  GetRadioLink: async () =>
    mode === 'link-error'
      ? { ...radioResult(), code: 'CONNECT_FAILED' }
      : radioResult({ link }),
  StartRadio: async () => {
    starts++;
    return radioResult({
      job: { id: 'synthetic-job', state: 'RUNNING', code: 'OK' },
    });
  },
  RadioStatus: async () =>
    radioResult({
      link,
      reading: radioReading,
      cpe_reading: cpeReading,
      ap_code: mode === 'link-ap-fail' ? 'CONNECT_FAILED' : 'OK',
      cpe_code: mode === 'link-cpe-fail' ? 'CONNECT_FAILED' : 'OK',
      job: { id: 'synthetic-job', state: 'SUCCEEDED', code: 'OK' },
    }),
  CancelRadio: async () => radioResult(),
  CancelRadioOperation: async () => radioResult(),
  ConfirmRadioFamily: async () =>
    radioResult({
      reading: radioReading,
      profile: {
        device_id: devices[0].id,
        family: radioReading.family,
        confirmed: true,
        warning: 15,
        critical: 20,
      },
    }),
};
Object.assign(app, radioApp);
Object.assign(window, { radioVisualStarts: () => starts });
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
        mode.startsWith('link-') ? (
          <RadioLinkPanel
            profile="X"
            activation="a"
            device={{ ...devices[0], id: 'concentrator', role: 'concentrator' }}
            observation="observation"
            row="row"
            pppoeUptime="2 h"
            blocked={false}
          />
        ) : (
          <Equipment
            profile="X"
            activation="a"
            connected
            createRequest={create}
            primaryInHeader
            onTotal={reportTotal}
          />
        )
      }
    />
  );
}
createRoot(document.getElementById('root')!).render(<Scene />);
