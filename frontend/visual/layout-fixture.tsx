import savedReadings from './layout-readings.json';
import {
  RadioRoles,
  RadioTransports,
  RadioFamilies,
} from '../src/radio-contract';
// Synthetic documentation addresses only. No production state or credentials are read.
import type { Device } from '../src/equipment-api';
const time = Date.parse('2026-11-04T12:00:00Z');
const entries = [
  ['CCR-CENTRO', 'concentrator', '192.0.2.1', '6.49', '12d4h', 8, 12],
  ['AP-NORTE', RadioRoles.ap, '192.0.2.20', '6.48', '30d1h', 14, 240],
  ['CPE-0001', RadioRoles.cpe, '192.0.2.101', '6.47', '3d7h', 5, 120],
  ['PTP-0001', RadioRoles.ptp, '192.0.2.50', '6.49', '45d2h', 3, 3600],
  ['AP-SUL', RadioRoles.ap, '192.0.2.21', '6.48', '', undefined, 86400 * 9],
  ['EQP-0001', 'other', '192.0.2.80', '', '', undefined, 0],
];
const devices = entries.map(
  ([name, role, address, version, uptime, cpu, age]) => ({
    id: name,
    name,
    role,
    address,
    port: 22,
    transport: RadioTransports.ssh,
    credential_id:
      name === 'CCR-CENTRO'
        ? 'concentrator'
        : name === 'PTP-0001'
          ? RadioRoles.ptp
          : 'radio',
    revision: 1,
    active: name !== 'AP-SUL',
    fingerprint: name === 'EQP-0001' ? '' : 'SHA256:xxxx…xxxx',
    trust_origin: 'first-connection',
    result: version
      ? {
          identity: name === 'CCR-CENTRO' ? name : 'id-genérico',
          family: 'v6',
          board: 'Modelo-X',
          version: `RouterOS ${version}`,
          uptime,
          cpu_percent: cpu,
          free_memory_bytes: 59 * 1024 ** 3,
          total_memory_bytes: 100 * 1024 ** 3,
          observed_at: new Date(time - Number(age) * 1000).toISOString(),
          source: 'SSH',
          quality: 'complete',
          interface_part: {
            status: 'complete',
            observed_at: new Date(time - 360000).toISOString(),
            counts_known: true,
            eligible: 2,
            dynamic: 23,
            total: 25,
            source: 'SSH',
            code: 'OK',
          },
          interfaces: [
            { name: 'ether1', kind: 'ether', running: true, disabled: false },
            { name: 'ether2', kind: 'ether', running: false, disabled: false },
          ],
        }
      : undefined,
  }),
) as Device[];
const credentials = [
  {
    id: 'concentrator',
    alias: 'leitura-concentrador',
    user: 'leitura-01',
    revision: 1,
    links: 1,
  },
  {
    id: 'radio',
    alias: 'leitura-radio',
    user: 'leitura-02',
    revision: 1,
    links: 3,
  },
  {
    id: RadioRoles.ptp,
    alias: 'leitura-ptp',
    user: 'leitura-03',
    revision: 1,
    links: 1,
  },
];
const sessions = Array.from({ length: 25 }, (_, i) => ({
  row_id: `row-${i}`,
  remote_id: `*${i}`,
  session_id: String(i),
  login: `cliente-${String(i + 1).padStart(4, '0')}`,
  address: `192.0.2.${201 + i}`,
  caller_id: `02:00:5E:00:00:${String(i + 1).padStart(2, '0')}`,
  state: (i + 1) % 7 === 0 ? 'OFFLINE' : 'ONLINE',
  uptime: {
    min: [2 * 86400 + 3 * 3600, 5 * 3600 + 12 * 60, 11 * 86400][i % 3] * 1e9,
    max: [2 * 86400 + 3 * 3600, 5 * 3600 + 12 * 60, 11 * 86400][i % 3] * 1e9,
    known: true,
  },
  duplicates: 1,
  context: {
    association: 'associated',
    profile: ['Plano 100M', 'Plano 50M', 'Plano 200M'][i % 3],
    secret_state: 'active',
    vlan: 100,
    service: 'pppoe-0001',
    interface: 'ether1',
    download_limit: '100000000',
    upload_limit: '20000000',
  },
}));
const current = {
  observation_id: 'observation',
  count: sessions.length,
  online: 22,
  offline: 3,
  provenance: { ended_at: new Date(time - 12000).toISOString() },
  quality: { status: 'complete' },
};
const metric = (name: string, value: number, unit: string) => ({
  name,
  value,
  unit,
  direction: 'local',
  perspective: 'AP',
  device_id: 'visual',
  interface: 'wlan1',
  observed_at: new Date(time - 240000).toISOString(),
  quality: 'complete',
});
function radioReading(deviceId: string) {
  const device = devices.find((d) => d.id === deviceId)!;
  return {
    id: 'radio-' + deviceId,
    device_id: deviceId,
    device_name: 'id-genérico',
    family: RadioFamilies.routerOSWirelessV6,
    revision: 1,
    credential_revision: 1,
    perspective: device.role === RadioRoles.cpe ? 'CPE' : 'AP',
    version: device.result?.version,
    model: 'Modelo-X',
    observed_at: device.result?.observed_at,
    compatible: true,
    connection: RadioTransports.ssh,
    seconds: 1,
    device_uptime: device.result?.uptime,
    had_clients: false,
    metrics: [
      metric('cpu', Number(device.result?.cpu_percent), '%'),
      metric('voltage', 24, 'V'),
      ...(device.role === RadioRoles.cpe
        ? [metric('temperature', 41, '°C')]
        : []),
      metric('signal', -58, 'dBm'),
      metric('snr', 42, 'dB'),
      metric('noise', -100, 'dBm'),
      metric('ccq', 92, '%'),
    ],
    interfaces: [
      {
        name: 'wlan1',
        bssid: '02:00:5E:00:00:01',
        band: '5GHz',
        configured_width: '20 MHz',
        channel: '5800 MHz',
        clients: 2,
        metrics: [metric('noise', -100, 'dBm')],
      },
    ],
    clients: [0, 1].map((i) => ({
      name: `CPE-000${i + 1}`,
      mac: `02:00:5E:00:00:0${i + 1}`,
      interface: 'wlan1',
      bssid: '02:00:5E:00:00:01',
      metrics: [
        metric('signal', i ? -66 : -58, 'dBm'),
        metric('ccq', i ? 80 : 92, '%'),
        metric('rate_tx', i ? 65 : 130, 'Mbps'),
        metric('rate_rx', i ? 58 : 117, 'Mbps'),
      ],
    })),
  };
}
const curve = [
  72,
  74,
  73,
  76,
  79,
  81,
  84,
  83,
  86,
  88,
  null,
  null,
  89,
  91,
  90,
  70,
  78,
  85,
  90,
  92,
  94,
  95,
  97,
  99,
];
function read(scope: string) {
  const at = new Date(time - 360000).toISOString();
  const extra = savedReadings as Record<string, object[]>;
  const wave = (i: number, base: number, amp: number) =>
    +(
      base +
      amp *
        (0.55 * Math.sin(i / 2.3) +
          0.3 * Math.sin(i / 1.1 + 1) +
          0.15 * Math.sin(i * 1.7))
    ).toFixed(2);
  const rows =
    extra[scope] ||
    (scope === 'traffic'
      ? [
          {
            kind: 'traffic',
            row_id: 'row-0',
            login: 'cliente-0001',
            traffic: {
              points: Array.from({ length: 30 }, (_, i) => ({
                t: i * 2000,
                gap: i === 16 || i === 17,
                d: wave(i, 11, 6) * 1e6,
                u: wave(i + 4, 1.1, 0.45) * 1e6,
              })),
              download_bytes: '432013312',
              upload_bytes: '38797312',
              covered_ns: 54e9,
              gaps: 1,
              average_download: { min: 98e5, max: 98e5 },
              average_upload: { min: 9e5, max: 9e5 },
              peak_download: { min: 182e5, max: 182e5 },
              peak_upload: { min: 16e5, max: 16e5 },
            },
          },
        ]
      : scope === 'series'
        ? curve
            .map((online, i) => ({
              kind: 'point',
              point: {
                at: new Date(
                  (1793800000 - 86400 + i * 3600) * 1000,
                ).toISOString(),
                resolution: 3600,
                online: online ?? undefined,
                online_min: online ?? undefined,
                online_max: online ?? undefined,
                samples: 1,
                gap: online == null,
                drop: i === 15,
              },
            }))
            .reverse()
        : []);
  return {
    scope,
    rows,
    started_at: new Date(time - 60000).toISOString(),
    ended_at: scope === 'traffic' ? new Date(time).toISOString() : at,
    pools_available: true,
    uplinks_available: true,
    pool_total: 1,
    uplink_total: 1,
    availability: {
      services: 'complete',
      packages: 'complete',
      interfaces: 'complete',
      pools: 'complete',
      auth: 'complete',
      fasttrack: 'complete',
      ports: 'complete',
    },
    duration_ns: 60e9,
  };
}
export function installFixture() {
  Date.now = () => time;
  const result = (data = {}) => ({
    code: 'OK',
    profile_id: 'visual',
    activation_id: 'fixture',
    data: {
      devices: [],
      credentials: [],
      vault_available: true,
      next_cursor: 0,
      ...data,
    },
  });
  Object.assign(window, {
    go: {
      presentation: {
        App: {
          ListEquipment: async (
            _p: string,
            _a: string,
            search: string,
            _cursor: number,
            inactive: boolean,
          ) =>
            result({
              devices: devices.filter(
                (d) => (inactive || d.active) && d.name.includes(search),
              ),
              total_devices: devices.length,
            }),
          GetEquipment: async (_p: string, _a: string, id: string) =>
            result({ device: devices.find((d) => d.id === id) }),
          ListCredentials: async () =>
            result({
              credentials,
              credential_defaults: {
                ap: 'radio',
                cpe: 'radio',
                [RadioRoles.ptp]: RadioRoles.ptp,
              },
            }),
          GetPPPoECurrent: async () => result({ current }),
          ListPPPoESessions: async (
            _p: string,
            _a: string,
            _d: string,
            _o: string,
            q: { search: string },
            cursor: string,
          ) => {
            const filtered = sessions.filter(
              (row) => !q.search || row.login.includes(q.search),
            );
            const start = Number(cursor) || 0;
            return result({
              rows: filtered.slice(start, start + 10),
              total: filtered.length,
              next_cursor:
                start + 10 < filtered.length ? String(start + 10) : '',
            });
          },
          GetPPPoESession: async (
            _p: string,
            _a: string,
            _d: string,
            _o: string,
            id: string,
          ) => result({ rows: sessions.filter((row) => row.row_id === id) }),
          ListPPPoEReferences: async () =>
            result({
              references: [
                {
                  reference_id: 'reference',
                  observation_id: 'old',
                  created_at: '2026-11-02T17:00:00Z',
                  count: 23,
                },
              ],
              total: 1,
            }),
          ComparePPPoE: async () =>
            result({
              deltas: [
                ...[1, 4].map((i) => ({
                  category: 'changed',
                  base: sessions[i],
                  target: sessions[i],
                  base_row: sessions[i].row_id,
                  target_row: sessions[i].row_id,
                  address_changed: i === 1,
                  caller_changed: i === 4,
                })),
                ...[
                  [8, 'ambiguous'],
                  [19, 'new'],
                  [29, 'absent'],
                ].map(([i, category]) => ({
                  category,
                  target: {
                    ...sessions[0],
                    login: `cliente-${String(Number(i) + 1).padStart(4, '0')}`,
                  },
                })),
              ],
              total: 5,
            }),
          GetRadioLink: async (
            _p: string,
            _a: string,
            _d: string,
            _o: string,
            row: string,
          ) =>
            result({
              link: row === 'row-0' ? { cpe_id: 'CPE-0001' } : undefined,
            }),
          PPPoEContextCounts: async () =>
            result({
              groups: [
                { service: 'pppoe-0001', vlan: 100, count: 23, peak_7d: 25 },
              ],
              total: 1,
            }),
          GetDeviceMonitoring: async (
            _p: string,
            _a: string,
            _d: string,
            scope: string,
          ) =>
            result({
              read: read(scope),
              total: scope === 'series' ? curve.length : 0,
              next_cursor: '',
            }),
          GetRadio: async (_p: string, _a: string, d: string) =>
            result({
              reading: radioReading(d),
              link:
                d === 'CPE-0001'
                  ? {
                      concentrator_id: 'CCR-CENTRO',
                      ap_id: 'AP-NORTE',
                      cpe_id: d,
                      login: 'cliente-0001',
                      mac: '02:00:5E:00:00:01',
                      interface: 'wlan1',
                      bssid: '02:00:5E:00:00:01',
                      confirmed_at: new Date(time - 120000).toISOString(),
                      confirmed_now: true,
                      origin: 'confirmed',
                      scope: 'fixture',
                    }
                  : undefined,
              total: 2,
              more: false,
              next_offset: 0,
              profile: {
                device_id: d,
                family: RadioFamilies.routerOSWirelessV6,
                confirmed: true,
                warning: 24,
                critical: 32,
              },
            }),
          GetPPPoEDiagnostic: async (
            _p: string,
            _a: string,
            _d: string,
            scope: string,
          ) =>
            result({
              diagnostic: {
                diagnostic_id: scope,
                device_id: 'CCR-CENTRO',
                scope,
                count: scope === 'radius' ? 2 : 3,
                provenance: {
                  ended_at: new Date(time - 1200000).toISOString(),
                  version: '6.49',
                },
                quality: { status: 'complete' },
              },
              total: 1,
            }),
          ListPPPoEDiagnostic: async (
            _p: string,
            _a: string,
            _d: string,
            scope: string,
          ) =>
            result({
              diagnostic_rows:
                scope === 'radius'
                  ? [
                      {
                        id: 'aaa',
                        kind: 'aaa',
                        use_radius: true,
                        accounting: true,
                      },
                      {
                        id: 'radius',
                        kind: 'radius',
                        address: '192.0.2.10',
                        services: 'ppp',
                        auth_port: 1812,
                        accounting_port: 1813,
                        disabled: false,
                        protocol: 'udp',
                        counters: {
                          pending: '0',
                          requests: '0',
                          accepts: '0',
                          rejects: '0',
                          resends: '0',
                          timeouts: '0',
                          bad_replies: '0',
                          last_rtt: { known: true, min: 4e6, max: 4e6 },
                        },
                      },
                    ]
                  : [
                      {
                        id: 'service',
                        kind: 'service',
                        name: 'pppoe-0001',
                        parent: 'ether1',
                        range: '',
                        disabled: false,
                        relation: 'resolved',
                      },
                      {
                        id: 'vlan',
                        kind: 'vlan',
                        name: 'vlan100',
                        parent: 'ether1',
                        vlan_id: 100,
                        disabled: false,
                      },
                      {
                        id: 'absent',
                        kind: 'ether',
                        name: 'pppoe-out7',
                        not_seen_since: '2026-11-04T10:12:00Z',
                      },
                    ],
              total: 1,
            }),
          StartPPPoERead: async (
            _p: string,
            _a: string,
            _d: string,
            _op: string,
            scope: string,
          ) => result({ job: { id: scope, state: 'RUNNING', code: 'OK' } }),
          PPPoEReadStatus: async (_p: string, _a: string, job: string) =>
            result({
              read: read(job),
              job: {
                id: job,
                state: job === 'traffic' ? 'RUNNING' : 'SUCCEEDED',
                code: 'OK',
              },
              total: 1,
            }),
          StopPPPoETraffic: async () =>
            result({
              read: read('traffic'),
              job: { id: 'traffic', state: 'CANCELLED', code: 'CANCELLED' },
              total: 1,
            }),
          CancelPPPoEReadOperation: async () => result(),
          CancelPPPoERead: async () => result(),
        },
      },
    },
  });
}
