import { useRef } from 'react';
import type { Device } from './equipment-api';
import {
  DeviceMonitoring,
  PPPoEDiagnostics,
  PPPoEReadPanel,
} from './pppoe-diagnostics';
import {
  PPPoECompareView,
  PPPoEPairDetail,
  PPPoEReferencesView,
  PPPoESessionDetail,
} from './pppoe-session-detail';
import {
  PPPoEObsActions,
  PPPoEPaginationActions,
  PPPoESessionFilters,
  PPPoESessionsTable,
} from './pppoe-sessions-view';
import type { PPPoERefs, PPPoETab } from './pppoe-state';
import { usePPPoEState } from './pppoe-state';
import './pppoe.css';
import { pppoeAPI as api } from './presentation';

const PPPOE_TABS: readonly [PPPoETab, string][] = [
  ['sessions', 'Sessões'],
  ['compare', 'Comparação'],
  ['references', 'Referências'],
  ['catalog', 'VLANs e serviços'],
  ['radius', 'RADIUS'],
  ['monitoring', 'Monitoramento'],
];

type State = ReturnType<typeof usePPPoEState>;

function PPPoEHeader(p: {
  s: State;
  primaryRef: React.RefObject<HTMLButtonElement | null>;
  device: Device;
  disabled: boolean;
  back: () => void;
}) {
  const { s, primaryRef, device, disabled } = p;
  return (
    <>
      <button onClick={p.back}>Voltar ao equipamento</button>
      <div className="pppoe-head">
        <div>
          <h1>Sessões PPPoE</h1>
          <p>{device.name} · Logins dos clientes</p>
        </div>
        {!s.initialFailed && (s.tab === 'sessions' || s.tab === 'compare') && (
          <button
            ref={primaryRef}
            className="primary"
            disabled={
              disabled || s.loading || !device.active || !device.fingerprint
            }
            onClick={s.update}
          >
            {s.job ? 'Atualizando…' : 'Atualizar sessões'}
          </button>
        )}
      </div>
      <div role="tablist" aria-label="Leitura PPPoE">
        {PPPOE_TABS.map(([id, label]) => (
          <button
            key={id}
            role="tab"
            aria-selected={s.tab === id}
            onClick={() => {
              s.setTab(id);
              s.setCursor('');
              s.setHistory([]);
            }}
          >
            {label}
          </button>
        ))}
      </div>
    </>
  );
}

function PPPoEStatusBanners({ s }: { s: State }) {
  const isObsTab =
    s.tab === 'sessions' || s.tab === 'compare' || s.tab === 'references';
  const isListTab = s.tab === 'sessions' || s.tab === 'compare';
  return (
    <>
      {s.job && (
        <button
          onClick={() =>
            void s.run(() => api.cancel(s.profile, s.activation, s.job))
          }
        >
          Cancelar atualização
        </button>
      )}
      {s.statusFailed && s.job && (
        <button onClick={() => s.retryStatus((n) => n + 1)}>
          Consultar estado novamente
        </button>
      )}
      {s.error && isObsTab && <p role="alert">{s.error}</p>}
      {s.notice && <p role="status">{s.notice}</p>}
      {s.attempt && isListTab && <p role="status">{s.attempt}</p>}
      {s.initialFailed && !s.loading && isObsTab && (
        <button
          className="primary"
          onClick={() => {
            s.setError('');
            s.bump((n) => n + 1);
          }}
        >
          Reabrir leitura armazenada
        </button>
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

function PPPoESessionsSection(p: {
  s: State;
  refs: PPPoERefs;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  disabled: boolean;
  openEquipment?: (id: string) => void;
}) {
  const { s, refs, detailTriggerRef, disabled } = p;
  if (!s.current) return null;
  const offlineCount = s.current.offline || 0;
  let onlineCount =
    s.current.online ?? Math.max(0, (s.current.count || 0) - offlineCount);
  if (onlineCount === 0 && offlineCount === 0 && s.current.count)
    onlineCount = s.current.count;
  return (
    <>
      <p className="pppoe-state-explanation">
        ONLINE: vista na última leitura concluída. OFFLINE: vista antes, mas
        ausente nessa leitura; mantém os últimos dados até reconectar ou por 24
        h.
      </p>
      <p>
        <strong>{onlineCount} ONLINE</strong> ·{' '}
        <strong>{offlineCount} OFFLINE</strong>
      </p>
      <PPPoESessionFilters {...s} searchControlRef={refs.searchControlRef} />
      {s.pageState === 'loading' && (
        <p role="status">Carregando página armazenada…</p>
      )}
      <PPPoESessionsTable
        rows={s.rows}
        showPlan={s.showPlan}
        fastTrack={s.fastTrack}
        contextOmitted={s.current.context_omitted}
        onDetail={(row, trigger) => {
          detailTriggerRef.current = trigger;
          void s.runDetail(
            () =>
              api.detail(
                s.profile,
                s.activation,
                s.device.id,
                s.current!.observation_id,
                row.row_id,
              ),
            (r) => {
              s.setPair(undefined);
              s.setDetailTab('traffic');
              s.select(r.data?.rows?.[0]);
            },
          );
        }}
      />
      {s.pageState === 'ready' && !s.rows.length && (
        <p>
          {s.current.count === 0
            ? 'Nenhuma sessão ativa nesta leitura completa.'
            : 'Nenhuma sessão corresponde aos filtros.'}
        </p>
      )}
      {s.selected && (
        <PPPoESessionDetail
          {...s}
          selected={s.selected}
          current={s.current}
          disabled={disabled}
          openEquipment={p.openEquipment}
          detailHeadingRef={refs.detailHeadingRef}
          detailTriggerRef={refs.detailTriggerRef}
          detailIntentRef={refs.detailIntentRef}
        />
      )}
    </>
  );
}

function PPPoEObsBody(p: {
  s: State;
  refs: PPPoERefs;
  disabled: boolean;
  openEquipment?: (id: string) => void;
}) {
  const { s, refs, disabled, openEquipment } = p;
  if (!s.current || (s.tab !== 'sessions' && s.tab !== 'compare')) return null;
  return (
    <>
      <PPPoEObsActions
        {...s}
        deviceId={s.device.id}
        current={s.current}
        disabled={disabled}
        exportControlRef={refs.exportControlRef}
        csvExportControlRef={refs.csvExportControlRef}
        lifeRef={refs.lifeRef}
        exportFocusRef={refs.exportFocusRef}
      />
      {s.tab === 'sessions' ? (
        <PPPoESessionsSection
          s={s}
          refs={refs}
          detailTriggerRef={refs.detailTriggerRef}
          disabled={disabled}
          openEquipment={openEquipment}
        />
      ) : (
        <PPPoECompareView {...s} />
      )}
      {s.pair && (
        <PPPoEPairDetail
          {...s}
          pair={s.pair}
          pairHeadingRef={refs.pairHeadingRef}
          detailTriggerRef={refs.detailTriggerRef}
          detailIntentRef={refs.detailIntentRef}
        />
      )}
      <PPPoEPaginationActions {...s} searchControlRef={refs.searchControlRef} />
    </>
  );
}

function PPPoETabContent(p: {
  s: State;
  refs: PPPoERefs;
  disabled: boolean;
  openEquipment?: (id: string) => void;
}) {
  const { s, refs, disabled, openEquipment } = p;
  return (
    <>
      {s.tab === 'monitoring' && (
        <DeviceMonitoring
          profile={s.profile}
          activation={s.activation}
          device={s.device}
        />
      )}
      {s.tab === 'sessions' &&
        !s.loading &&
        !s.initialFailed &&
        !s.selected && (
          <PPPoEReadPanel
            key={
              (s.current?.observation_id || 'none') +
              '|' +
              s.rows.map((r) => r.row_id).join(',')
            }
            profile={s.profile}
            activation={s.activation}
            device={s.device}
            observation={s.current?.observation_id || ''}
            rows={
              s.current
                ? s.rows
                    .filter((r) => r.state !== 'OFFLINE')
                    .map((r) => r.row_id)
                : []
            }
            blocked={disabled}
            allowTop
          />
        )}
      {(s.tab === 'catalog' || s.tab === 'radius') && (
        <PPPoEDiagnostics
          key={s.context + '|' + s.tab}
          profile={s.profile}
          activation={s.activation}
          device={s.device}
          scope={s.tab}
          blocked={disabled}
        />
      )}
      <PPPoEObsBody
        s={s}
        refs={refs}
        disabled={disabled}
        openEquipment={openEquipment}
      />
      {s.tab === 'references' && (
        <PPPoEReferencesView
          {...s}
          disabled={disabled}
          referencesHeadingRef={refs.referencesHeadingRef}
          savedReferences={s.savedReferences}
        />
      )}
    </>
  );
}

export function PPPoE({
  profile,
  activation,
  device,
  back,
  openEquipment,
}: {
  profile: string;
  activation: string;
  device: Device;
  back: () => void;
  openEquipment?: (id: string) => void;
}) {
  const detailTriggerRef = useRef<HTMLElement | null>(null);
  const detailHeadingRef = useRef<HTMLHeadingElement>(null);
  const pairHeadingRef = useRef<HTMLHeadingElement>(null);
  const focusedControlRef = useRef<HTMLElement | null>(null);
  const searchControlRef = useRef<HTMLInputElement>(null);
  const referencesHeadingRef = useRef<HTMLHeadingElement>(null);
  const exportControlRef = useRef<HTMLButtonElement>(null);
  const csvExportControlRef = useRef<HTMLButtonElement>(null);
  const removedFocusRef = useRef('');
  const exportFocusRef = useRef<'sanitized' | 'operational' | ''>('');
  const lifeRef = useRef({ generation: 0 });
  const primaryRef = useRef<HTMLButtonElement>(null);
  const detailIntentRef = useRef(0);
  const refs: PPPoERefs = {
    detailTriggerRef,
    detailHeadingRef,
    pairHeadingRef,
    focusedControlRef,
    searchControlRef,
    referencesHeadingRef,
    exportControlRef,
    csvExportControlRef,
    removedFocusRef,
    exportFocusRef,
    lifeRef,
    primaryRef,
    detailIntentRef,
  };
  const s = usePPPoEState(profile, activation, device, refs);
  const disabled = s.busy || Boolean(s.job);
  return (
    <section
      className="pppoe-workspace"
      onFocusCapture={(event) => {
        focusedControlRef.current = event.target as HTMLElement;
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && (s.selected || s.pair)) {
          ++detailIntentRef.current;
          s.select(undefined);
          s.setPair(undefined);
          if (detailTriggerRef.current?.isConnected)
            detailTriggerRef.current.focus();
          event.preventDefault();
        }
      }}
      aria-label="Diagnóstico PPPoE"
      tabIndex={0}
    >
      <PPPoEHeader
        s={s}
        primaryRef={primaryRef}
        device={device}
        disabled={disabled}
        back={back}
      />
      <PPPoEStatusBanners s={s} />
      <PPPoETabContent
        s={s}
        refs={refs}
        disabled={disabled}
        openEquipment={openEquipment}
      />
    </section>
  );
}
