import type { Props } from './equipment-detail';
import { EquipmentSummaryStatistics } from './equipment-summary-statistics';
import { EquipmentReading } from './equipment-reading';
import { DataBlock } from './ui/DataBlock';
import { useRefreshOnOpen } from './use-refresh-on-open';

export function equipmentReadBlocked(p: Props) {
  return (
    p.locked ||
    !p.vault ||
    !p.selected.active ||
    !p.selected.credential_id ||
    p.job?.state === 'RUNNING'
  );
}

export function useSummaryRefresh(p: Props, tab: string, isRadio: boolean) {
  useRefreshOnOpen({
    opening: !isRadio && tab === 'Resumo' ? `${p.selected.id}|Resumo` : '',
    scope: `${p.profile}|${p.activation}`,
    ready: p.ready,
    input: {
      observedAt: p.selected.result?.observed_at,
      device: p.selected,
      vaultAvailable: p.vault,
      serviceAvailable: p.connected,
      reading: p.busy || p.job?.state === 'RUNNING',
    },
    refresh: () => void p.startOperation('query', p.selected),
  });
}

export function SummaryTab({ p }: { p: Props }) {
  return p.selected.result ? (
    <EquipmentReading
      device={p.selected}
      focus={false}
      section="summary"
      status={{
        refreshing: p.job?.state === 'RUNNING' || p.busy,
        error: p.error,
        code: p.job?.code,
        retry: () => void p.startOperation('query', p.selected),
        disabled: equipmentReadBlocked(p),
      }}
      statistics={
        <EquipmentSummaryStatistics
          profile={p.profile}
          activation={p.activation}
          device={p.selected}
        />
      }
    />
  ) : (
    <>
      {(!p.selected.role || p.selected.role === 'other') && (
        <div className="note">
          Declare o tipo deste equipamento para ver o restante.
        </div>
      )}
      <div className="grid">
        <DataBlock
          title="Identidade e sistema"
          source="SSH"
          data={{
            state: p.job?.state === 'RUNNING' ? 'loading' : 'unread',
            action: () =>
              void p.startOperation(
                p.selected.fingerprint ? 'query' : 'inspect',
                p.selected,
              ),
            disabled: equipmentReadBlocked(p),
          }}
        />
      </div>
    </>
  );
}
