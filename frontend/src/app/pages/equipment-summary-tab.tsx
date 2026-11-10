import type { Props } from './equipment-detail';
import { EquipmentReading } from '../../features/equipment/equipment-reading';
import { queryFailureOwnsDetail } from '../../features/equipment/equipment-query-failure';
import { EquipmentSummaryStatistics } from '../../features/equipment/equipment-summary-statistics';
import { equipmentReadBlocked } from './equipment-query-policy';
import { DataBlock } from '../../shared/ui/DataBlock';

export function SummaryTab({ p }: { p: Props }) {
  return p.selected.result ? (
    <EquipmentReading
      device={p.selected}
      focus={false}
      section="summary"
      status={{
        refreshing: p.job?.state === 'RUNNING' || p.busy,
        error: queryFailureOwnsDetail(p) ? p.error : undefined,
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
