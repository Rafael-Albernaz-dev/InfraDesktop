import { readingAge } from '../equipment/equipment-reading-format';
import { ContextCounts } from './pppoe-context';
import { CSVNotice } from './pppoe-csv-notice';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { PPPoEHeader, PPPoEStatusBanners } from './pppoe-header';
import { observationReadingStatus } from './pppoe-observation-state';
import { PPPoETabContent } from './pppoe-observation-view';
import type { PPPoETab } from './pppoe-state';
import { readingUpdateState } from '../monitoring/reading-update-state';
import { DataBlock } from '../../shared/ui/DataBlock';
const PPPOE_TABS: readonly [PPPoETab, string][] = [
  ['sessions', 'Sessões'],
  ['compare', 'Comparação'],
  ['references', 'Referências'],
  ['catalog', 'VLANs e serviços'],
  ['radius', 'RADIUS'],
  ['monitoring', 'Monitoramento'],
];
type WorkspaceProps = Parameters<typeof PPPoETabContent>[0] & {
  section: 'sessions' | 'services';
};
function ObservationBlock(p: WorkspaceProps) {
  const {
    s,
    refs,
    disabled,
    openEquipment,
    onOpenSession: openSession,
    onFocusOrigin: rememberFocus,
  } = p;
  if (s.tab === 'catalog' || s.tab === 'radius')
    return <PPPoETabContent {...p} />;
  return (
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
  );
}
function SessionsReadPanels({ s, disabled }: WorkspaceProps) {
  return (
    <>
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
    </>
  );
}
export function PPPoEWorkspace(p: WorkspaceProps) {
  const { s, refs, section } = p;
  const { focusedControlRef, detailIntentRef, detailTriggerRef } = refs;
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
      <ObservationBlock {...p} />
      <SessionsReadPanels {...p} />
    </section>
  );
}
