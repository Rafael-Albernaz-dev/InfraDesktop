// Isolated visual fixture only; the runtime imports no file from frontend/visual.
import { createRoot } from 'react-dom/client';
import { Desktop } from '../src/desktop';
import { PPPoE } from '../src/pppoe';
import type { Device } from '../src/equipment-api';
import '../src/style.css';
const params = new URLSearchParams(location.search);
const mode = params.get('state') || 'current';
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
