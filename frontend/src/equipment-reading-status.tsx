import type { Device, Job } from './equipment-api';
import { equipmentMessage } from './equipment-api';
type ReadingOperation = {
  detailOpen: boolean;
  selected?: Pick<Device, 'id' | 'result'>;
  job?: Job;
  error: string;
};
export function queryFailureOwnsDetail(p: ReadingOperation): boolean {
  return Boolean(
    p.detailOpen &&
    p.selected?.result &&
    p.job?.device_id === p.selected.id &&
    p.job?.kind === 'device.query-start' &&
    p.job.state === 'FAILED' &&
    p.error &&
    p.error === equipmentMessage(p.job.code),
  );
}
export function EquipmentQueryFailure({
  state,
  tab,
  isRadio,
}: {
  state: ReadingOperation;
  tab: string;
  isRadio: boolean;
}) {
  if (!queryFailureOwnsDetail(state) || (!isRadio && tab === 'Resumo'))
    return null;
  return <p role="alert">{state.error}</p>;
}
