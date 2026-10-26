// Isolated visual fixture only; the runtime imports no file from frontend/visual.
import { createRoot } from 'react-dom/client';
import { Desktop } from '../src/desktop';
import { PPPoE } from '../src/pppoe';
import type { Device } from '../src/equipment-api';
import '../src/style.css';
const params = new URLSearchParams(location.search);
const mode = params.get('state') || 'current';
const diagnosticState = params.get('diagnostic') || 'ready';
const diagnosticRows = (scope: string) =>
  scope === 'catalog'
    ? [
        {
          id: 'if-1',
          kind: 'interface',
          name: 'ether1',
          type: 'ether',
          running: true,
          disabled: false,
          dynamic: false,
        },
        {
          id: 'vlan-1',
          kind: 'vlan',
          name: 'VLAN externa',
          parent: 'ether1',
          relation: 'resolved',
          vlan_id: 100,
          service_tag: true,
          running: true,
          disabled: false,
        },
        {
          id: 'vlan-2',
          kind: 'vlan',
          name: 'VLAN interna',
          parent: 'VLAN externa',
          relation: 'resolved',
          vlan_id: 200,
          service_tag: false,
          running: true,
          disabled: false,
        },
        {
          id: 'service-1',
          kind: 'service',
          name: 'Serviço PPPoE sintético',
          parent: 'VLAN interna',
          relation: diagnosticState === 'ambiguous' ? 'ambiguous' : 'resolved',
          range: '100-115,120',
          accept_untagged: true,
          disabled: false,
        },
      ]
    : scope === 'radius'
      ? [
          { id: 'aaa', kind: 'aaa', use_radius: true, accounting: true },
          {
            id: 'radius-1',
            kind: 'radius',
            address: '198.51.100.20',
            services: 'ppp',
            protocol: 'udp',
            disabled: false,
            auth_port: 1812,
            accounting_port: 1813,
            counters: {
              pending: '0',
              requests: '100',
              accepts: '90',
              rejects: '3',
              resends: '2',
              timeouts: '1',
              bad_replies: '0',
              last_rtt: { known: true, min: 1045000, max: 1045999 },
            },
          },
        ]
      : [
          {
            id: 'ping-row',
            kind: 'ping',
            ping: {
              sent: 3,
              received: diagnosticState === 'no-reply' ? 0 : 3,
              loss_percent: diagnosticState === 'no-reply' ? 100 : 0,
              min_ns: diagnosticState === 'no-reply' ? undefined : 1045000,
              avg_ns: diagnosticState === 'no-reply' ? undefined : 1099000,
              max_ns: diagnosticState === 'no-reply' ? undefined : 1163000,
              context: 'concentrador · main',
              probes: [0, 1, 2].map((sequence) => ({
                sequence,
                status: diagnosticState === 'no-reply' ? 'timeout' : '',
                rtt_ns: diagnosticState === 'no-reply' ? undefined : 1045000,
              })),
            },
          },
        ];
const rows = Array.from({ length: 16 }, (_, i) => ({
  row_id: 'row-' + i,
  login: 'Operação sintética ' + i,
  address: '198.51.100.' + (i + 1),
  caller_id: '02:00:00:00:00:01',
  duplicates: 2,
  uptime: { known: true, min: 1e9, max: 2e9 },
}));
const refs = Array.from({ length: 8 }, (_, i) => ({
  reference_id: 'ref-' + i,
  observation_id: 'observation-' + i,
  created_at: '2026-10-25T00:00:00Z',
  count: 16,
}));
const result = (data: unknown) => ({
  code: 'OK',
  profile_id: 'X',
  activation_id: 'a',
  data,
});
Object.assign(window, {
  go: {
    presentation: {
      App: {
        GetPPPoECurrent: async () => {
          if (mode === 'uncollected') return result({});
          if (mode === 'error')
            return {
              code: 'SERVICE_UNAVAILABLE',
              profile_id: 'X',
              activation_id: 'a',
            };
          if (mode === 'loading') return new Promise(() => {});
          return result({
            attempt: ['limited', 'partial'].includes(mode)
              ? {
                  state: 'FAILED',
                  code: mode === 'limited' ? 'LIMIT_EXCEEDED' : 'PARTIAL',
                  quality: {
                    status: mode === 'limited' ? 'limited' : 'partial',
                    counts_known: true,
                    eligible: mode === 'limited' ? 50000 : 16,
                  },
                }
              : undefined,
            current: {
              observation_id: 'current',
              count: mode === 'empty' ? 0 : 16,
              provenance: { ended_at: '2026-10-25T00:00:00Z' },
              quality: { status: 'complete' },
            },
          });
        },
        ListPPPoEReferences: async () =>
          result({ references: mode === 'uncollected' ? [] : refs }),
        ListPPPoESessions: async (...args: unknown[]) =>
          mode === 'page-error'
            ? { ...result({}), code: 'SERVICE_UNAVAILABLE' }
            : result({
                rows:
                  mode === 'empty' ||
                  (mode === 'no-result' &&
                    Boolean((args[4] as { search?: string })?.search))
                    ? []
                    : rows,
                total:
                  mode === 'empty' ||
                  (mode === 'no-result' &&
                    Boolean((args[4] as { search?: string })?.search))
                    ? 0
                    : 16,
              }),
        GetPPPoESession: async () => result({ rows: [rows[0]] }),
        GetPPPoEDiagnostic: async (
          _p: string,
          _a: string,
          _d: string,
          scope: string,
        ) => {
          if (diagnosticState === 'loading') return new Promise(() => {});
          if (diagnosticState === 'error')
            return { ...result({}), code: 'SERVICE_UNAVAILABLE' };
          if (diagnosticState === 'uncollected') return result({});
          const failed = ['partial', 'limited', 'stale'].includes(
            diagnosticState,
          );
          return result({
            diagnostic: {
              diagnostic_id: scope + '-snapshot',
              device_id: 'device',
              scope,
              count:
                diagnosticState === 'empty'
                  ? scope === 'radius'
                    ? 1
                    : 0
                  : diagnosticRows(scope).length,
              provenance: { ended_at: '2026-10-25T00:00:00Z', version: '7.26' },
              quality: { status: 'complete' },
              source_observation: scope === 'ping' ? 'current' : undefined,
              source_row: scope === 'ping' ? 'row-0' : undefined,
            },
            attempt: failed
              ? {
                  state: 'FAILED',
                  code:
                    diagnosticState === 'limited'
                      ? 'LIMIT_EXCEEDED'
                      : diagnosticState === 'stale'
                        ? 'DEADLINE_EXCEEDED'
                        : 'PARTIAL',
                  source_observation: 'current',
                  source_row: 'row-0',
                  quality: {
                    status:
                      diagnosticState === 'limited' ? 'limited' : 'partial',
                  },
                }
              : undefined,
          });
        },
        ListPPPoEDiagnostic: async (
          _p: string,
          _a: string,
          _d: string,
          scope: string,
        ) =>
          result({
            diagnostic_rows:
              diagnosticState === 'empty'
                ? scope === 'radius'
                  ? diagnosticRows(scope).slice(0, 1)
                  : []
                : diagnosticRows(scope),
            total:
              diagnosticState === 'empty'
                ? scope === 'radius'
                  ? 1
                  : 0
                : diagnosticRows(scope).length,
          }),
        StartPPPoECatalog: async () => ({
          ...result({}),
          code: 'REMOTE_PERMISSION_DENIED',
        }),
        StartPPPoERadius: async () => ({
          ...result({}),
          code: 'REMOTE_PERMISSION_DENIED',
        }),
        StartPPPoEPing: async () => ({
          ...result({}),
          code: 'OBSERVATION_INCOMPATIBLE',
        }),
      },
    },
  },
});
createRoot(document.getElementById('root')!).render(
  <Desktop
    page={4}
    navigate={() => {}}
    brand={{ name: 'Teste sintético', accent: 'teal', logo: '' }}
    account="Operador sintético"
    theme={params.get('theme') || 'light'}
    setTheme={() => {}}
    connected
    busy={false}
    companies={[]}
    activeProfile="X"
    create={() => {}}
    enter={() => {}}
    logout={() => {}}
    unavailable={() => {}}
    inventoryDetailOpen
    inventory={
      <PPPoE
        profile="X"
        activation="a"
        device={
          {
            id: 'device',
            name: 'Concentrador sintético',
            active: true,
            revision: 2,
            fingerprint: 'test',
          } as Device
        }
        back={() => {}}
      />
    }
  />,
);
