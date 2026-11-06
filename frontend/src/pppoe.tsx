import { observationReadingStatus } from './pppoe-observation-state';
import { useOpenLinkedSession } from './linked-session-navigation';
import type { LinkedSessionTarget } from './linked-session-navigation';
import { useSessionsTabRefresh } from './automatic-tab-refresh';
import { readingUpdateState } from './reading-update-state';
import { ContextCounts } from './pppoe-context';
import { readingAge } from './equipment-reading-format';
import { CSVNotice } from './pppoe-csv-notice';
import { PPPoEHeader, PPPoEStatusBanners } from './pppoe-header';
import { useContext, useEffect, useRef } from 'react';
import { SessionRoute } from './session-navigation';
import type { PPRow } from './presentation';
import type { Device } from './equipment-api';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { PPPoETabContent } from './pppoe-observation-view';
import { PPPoESessionDetail } from './pppoe-session-screen';
import type { PPPoERefs, PPPoETab } from './pppoe-state';
import { usePPPoEState } from './pppoe-state';
import { DataBlock } from './ui/DataBlock';
import { Button } from './ui/Button';
const PPPOE_TABS: readonly [PPPoETab, string][] = [
  ['sessions', 'Sessões'],
  ['compare', 'Comparação'],
  ['references', 'Referências'],
  ['catalog', 'VLANs e serviços'],
  ['radius', 'RADIUS'],
  ['monitoring', 'Monitoramento'],
];
export function PPPoE({
  profile,
  activation,
  device,
  openEquipment,
  section = 'sessions',
  refreshAvailable = false,
  sessionTarget,
  onSessionConsumed,
}: {
  profile: string;
  activation: string;
  device: Device;
  back: () => void;
  section?: 'sessions' | 'services';
  refreshAvailable?: boolean;
  sessionTarget?: LinkedSessionTarget;
  onSessionConsumed?: () => void;
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
  const navigation = useContext(SessionRoute);
  const wasActive = useRef(false);
  const returnFocus = useRef<{ container: Element; label: string } | undefined>(undefined);
  const active = navigation ? navigation.active : !!s.selected;
  const openSession = (row: PPRow, originDevice?: string) => {
    if (!s.current) return;
    navigation?.open({
      device: device.id,
      observation: s.current.observation_id,
      row: row.row_id,
      login: row.login,
      ...(originDevice ? { originDevice } : {}),
    });
  };
  useEffect(() => {
    if (wasActive.current && !active) {
      s.select(undefined);
      const origin = returnFocus.current;
      queueMicrotask(() => {
        const focused = origin?.container.ownerDocument.activeElement;
        if (focused && focused !== origin?.container.ownerDocument.body &&
            focused.isConnected && origin?.container.contains(focused)) return;
        const button = Array.from(origin?.container.querySelectorAll('button') || [])
          .find((element) => element.getAttribute('aria-label') === origin?.label);
        if (button) button.focus();
        else searchControlRef.current?.focus();
      });
    }
    wasActive.current = active;
  }, [active]);
  const rememberFocus = (element: HTMLElement) => {
    const container = element.closest('.pppoe-workspace')?.parentElement;
    const label = element.getAttribute('aria-label');
    if (container && label) returnFocus.current = { container, label };
  };
  useSessionsTabRefresh(s, refreshAvailable);
  const restoredTarget = navigation?.target;
  const selectedMatches = !restoredTarget || (
    restoredTarget.device === device.id &&
    restoredTarget.observation === s.current?.observation_id &&
    restoredTarget.row === s.selected?.row_id &&
    restoredTarget.login === s.selected?.login
  );
  const targetToLoad = restoredTarget && !selectedMatches ? {
    deviceID: restoredTarget.device,
    observation: restoredTarget.observation,
    row: restoredTarget.row,
  } : !navigation?.active ? sessionTarget : undefined;
  useOpenLinkedSession(s, targetToLoad, (row) => {
    if (navigation?.active && navigation.target &&
        (row.row_id !== navigation.target.row || row.login !== navigation.target.login)) {
      s.select(undefined);
      s.setError('A sessão retornada não corresponde ao alvo solicitado.');
      return;
    }
    if (!navigation?.active) {
      onSessionConsumed?.();
      openSession(row, targetToLoad?.originDevice);
    }
  });
  const disabled = s.busy || Boolean(s.job);
  if (active && !selectedMatches) {
    return <section aria-label="Detalhes da sessão">
      <p role={s.error ? 'alert' : 'status'}>{s.error || 'Carregando sessão…'}</p>
      <Button onClick={() => navigation?.back()}>Voltar</Button>
    </section>;
  }
  if (active && s.selected && s.current) {
    return <PPPoESessionDetail
      {...s}
      selected={s.selected}
      current={s.current}
      disabled={disabled}
      openEquipment={(id) => { navigation?.leave(); openEquipment?.(id); }}
      detailHeadingRef={detailHeadingRef}
      detailTriggerRef={detailTriggerRef}
      detailIntentRef={detailIntentRef}
      back={() => navigation?.back()}
    />;
  }
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
          onOpenSession={openSession}
          onFocusOrigin={rememberFocus}
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
          data={readingUpdateState(
            <PPPoETabContent
              s={s}
              refs={refs}
              disabled={disabled}
              openEquipment={openEquipment}
              onOpenSession={openSession}
              onFocusOrigin={rememberFocus}
            />,
            observationReadingStatus(s, disabled),
          )}
        />
      )}
      {s.tab === 'sessions' && !s.current && <CSVNotice />}
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
