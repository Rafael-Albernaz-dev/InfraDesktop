import type { ReactNode } from 'react';
import type { DataState } from '../../shared/ui/DataBlock';
export type ReadingUpdateStatus = {
  refreshing: boolean;
  error?: string;
  code?: string;
  retry: () => void;
  disabled?: boolean;
};
export function readingUpdateState(
  content: ReactNode,
  status?: ReadingUpdateStatus,
): DataState {
  if (status?.refreshing) return { state: 'refreshing', content };
  if (status?.error)
    return {
      state: 'failed',
      previous: content,
      reason: status.error,
      technical: status.code || 'Falha na atualização',
      retry: status.retry,
      disabled: status.disabled,
    };
  return { state: 'ready', content };
}
