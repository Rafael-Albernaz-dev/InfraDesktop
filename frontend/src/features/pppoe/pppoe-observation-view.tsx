import type { PPRow } from '../../platform/wails/presentation';
import type { ReactNode } from 'react';

import { pppoeAPI as api } from '../../platform/wails/presentation';
import { CSVNotice } from './pppoe-csv-notice';
import { DeviceMonitoring, PPPoEDiagnostics } from './pppoe-diagnostics';
import {
  PPPoECompareView,
  PPPoEPairDetail,
  PPPoEReferencesView,
} from './pppoe-session-detail';
import {
  PPPoEObsActions,
  PPPoEPaginationActions,
  PPPoESessionFilters,
  PPPoESessionsTable,
} from './pppoe-sessions-view';
import { usePPPoEState } from './pppoe-state';
import type { PPPoERefs } from './pppoe-state';

type State = ReturnType<typeof usePPPoEState>;
function PPPoESessionsSection(p: {
  s: State;
  refs: PPPoERefs;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  disabled: boolean;
  openEquipment?: (id: string) => void;
  actions: ReactNode;
  onOpenSession: (row: PPRow) => void;
  onFocusOrigin: (element: HTMLElement) => void;
}) {
  const { s, refs, detailTriggerRef } = p;
  if (!s.current) return null;
  return (
    <>
      <PPPoESessionFilters {...s} searchControlRef={refs.searchControlRef} />
      {p.actions}
      <CSVNotice />
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
          p.onFocusOrigin(trigger);
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
              if (r.data?.rows?.[0]) p.onOpenSession(r.data.rows[0]);
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
    </>
  );
}
function PPPoEObsBody(p: {
  s: State;
  refs: PPPoERefs;
  disabled: boolean;
  openEquipment?: (id: string) => void;
  onOpenSession: (row: PPRow) => void;
  onFocusOrigin: (element: HTMLElement) => void;
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
          onOpenSession={p.onOpenSession}
          onFocusOrigin={p.onFocusOrigin}
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
export function PPPoETabContent(p: {
  s: State;
  refs: PPPoERefs;
  disabled: boolean;
  openEquipment?: (id: string) => void;
  onOpenSession: (row: PPRow) => void;
  onFocusOrigin: (element: HTMLElement) => void;
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
        onOpenSession={p.onOpenSession}
        onFocusOrigin={p.onFocusOrigin}
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
