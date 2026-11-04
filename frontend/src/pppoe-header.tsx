import { showSeparatePPPoEError } from './pppoe-observation-state';
import { Button } from './ui/Button';
import { HeaderAction } from './ui/HeaderAction';
import type { PPPoETab } from './pppoe-state';
import { usePPPoEState } from './pppoe-state';
import { Tabs } from './ui/Tabs';

import { pppoeAPI as api } from './presentation';
type State = ReturnType<typeof usePPPoEState>;
export function PPPoEHeader({
  s,
  section,
}: {
  s: State;
  section: 'sessions' | 'services';
}) {
  const items: [PPPoETab, string][] =
    section === 'services'
      ? [
          ['catalog', 'VLANs e serviços'],
          ['radius', 'RADIUS'],
        ]
      : [
          ['sessions', 'Lista'],
          ['compare', 'Comparação'],
          ['references', 'Referências'],
        ];
  return (
    <>
      {section === 'sessions' && !s.initialFailed && (
        <HeaderAction>
          <Button
            kind="primary"
            disabled={
              s.job
                ? s.busy
                : s.busy ||
                  s.loading ||
                  !s.device.active ||
                  !s.device.fingerprint
            }
            onClick={() =>
              s.job
                ? void s.run(() => api.cancel(s.profile, s.activation, s.job))
                : s.update()
            }
          >
            {s.job ? 'Parar' : 'Atualizar'}
          </Button>
        </HeaderAction>
      )}
      <Tabs
        label={section === 'services' ? 'Serviços' : 'Sessões'}
        sub
        items={items}
        value={s.tab}
        onChange={(id) => {
          s.setTab(id);
          s.setCursor('');
          s.setHistory([]);
        }}
      />
    </>
  );
}
export function PPPoEStatusBanners({ s }: { s: State }) {
  const isObsTab =
    s.tab === 'sessions' || s.tab === 'compare' || s.tab === 'references';
  const isListTab = s.tab === 'sessions' || s.tab === 'compare';
  return (
    <>
      {s.statusFailed && s.job && (
        <Button type="submit" onClick={() => s.retryStatus((n) => n + 1)}>
          Conferir andamento
        </Button>
      )}
      {showSeparatePPPoEError(s) && isObsTab && <p role="alert">{s.error}</p>}
      {s.notice && <p role="status">{s.notice}</p>}
      {s.attempt && isListTab && <p role="status">{s.attempt}</p>}
      {s.initialFailed && !s.loading && isObsTab && (
        <Button
          type="submit"
          className="primary"
          onClick={() => {
            s.setError('');
            s.bump((n) => n + 1);
          }}
        >
          Reabrir leitura armazenada
        </Button>
      )}
      {s.loading && isListTab && (
        <p role="status">Carregando leitura armazenada…</p>
      )}
      {!s.loading && !s.initialFailed && !s.current && isListTab && (
        <p>
          Nenhuma leitura de sessões disponível. Atualize para consultar este
          concentrador.
        </p>
      )}
    </>
  );
}
