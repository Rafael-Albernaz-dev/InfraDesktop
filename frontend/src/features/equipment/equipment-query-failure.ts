import type { Device, Job } from './equipment-api';
import { equipmentMessage } from './equipment-api';

export type ReadingOperation = {
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
