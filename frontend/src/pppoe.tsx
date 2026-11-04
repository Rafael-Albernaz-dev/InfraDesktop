import type { ReactNode } from 'react';
import { ContextCounts } from './pppoe-context';
import { readingAge } from './equipment-reading-format';
import { PPPoEHeader, PPPoEStatusBanners } from './pppoe-header';

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

import { DataBlock } from './ui/DataBlock';
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

function PPPoESessionsSection(p: {
  s: State;
  refs: PPPoERefs;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  disabled: boolean;
  openEquipment?: (id: string) => void;
  actions: ReactNode;
}) {
  const { s, refs, detailTriggerRef, disabled } = p;
  if (!s.current) return null;
  return (
    <>
      <PPPoESessionFilters {...s} searchControlRef={refs.searchControlRef} />
      {p.actions}
      {s.pageState === 'loading' && (
        <p role="status">Carregando página armazenada…</p>
      )}
      <PPPoESessionsTable
        rows={s.rows}
        profile={s.profile}
        activation={s.activation}
        deviceId={s.device.id}
        observation={s.current.observation_id}
        openEquipment={p.openEquipment}
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
      <PPPoEPaginationActions {...s} searchControlRef={refs.searchControlRef} />
      <p className="muted">
        ONLINE: vista na última leitura concluída. OFFLINE: vista antes e
        ausente nesta leitura.
      </p>
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
      {s.tab === 'sessions' ? (
        <PPPoESessionsSection
          s={s}
          refs={refs}
          detailTriggerRef={refs.detailTriggerRef}
          disabled={disabled}
          openEquipment={openEquipment}
          actions={
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
          }
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
      {s.tab === 'compare' && (
        <PPPoEPaginationActions
          {...s}
          searchControlRef={refs.searchControlRef}
        />
      )}
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
  openEquipment,
  section = 'sessions',
}: {
  profile: string;
  activation: string;
  device: Device;
  back: () => void;
  section?: 'sessions' | 'services';
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
  const s = usePPPoEState(
    profile,
    activation,
    device,
    refs,
    section === 'services' ? 'catalog' : 'sessions',
  );
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
      <PPPoEHeader s={s} section={section} />
      <PPPoEStatusBanners s={s} />
      {s.tab === 'catalog' || s.tab === 'radius' ? (
        <PPPoETabContent
          s={s}
          refs={refs}
          disabled={disabled}
          openEquipment={openEquipment}
        />
      ) : (
        <DataBlock
          title={
            s.tab === 'sessions'
              ? 'Sessões PPPoE'
              : PPPOE_TABS.find(([id]) => id === s.tab)?.[1] || 'Sessões'
          }
          source={s.tab === 'references' ? 'cadastro local' : 'SSH'}
          age={readingAge(
            s.tab === 'references'
              ? s.savedReferences[0]?.created_at
              : s.current?.provenance.ended_at,
          )}
          data={{
            state: 'ready',
            content: (
              <PPPoETabContent
                s={s}
                refs={refs}
                disabled={disabled}
                openEquipment={openEquipment}
              />
            ),
          }}
        />
      )}
      {s.tab === 'sessions' && !s.loading && !s.initialFailed && (
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
              ? s.rows.filter((r) => r.state !== 'OFFLINE').map((r) => r.row_id)
              : []
          }
          blocked={disabled}
          allowTop
          pageBandwidth
        />
      )}
      {s.tab === 'sessions' && s.current && !s.current.context_omitted && (
        <>
          <div className="sp10" />
          <ContextCounts
            key={s.current.observation_id}
            profile={s.profile}
            activation={s.activation}
            device={s.device.id}
            observation={s.current.observation_id}
            at={s.current.provenance.ended_at}
          />
        </>
      )}
    </section>
  );
}
