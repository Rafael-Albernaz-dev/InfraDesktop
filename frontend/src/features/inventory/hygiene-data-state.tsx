import type { PPReadRow } from '../../platform/wails/presentation';
import type { DataState } from '../../shared/ui/DataBlock';
export function hygieneDataState(
  rows: PPReadRow[],
  next: string,
  retry: () => void,
): DataState | undefined {
  if (rows.some((r) => r.kind === 'availability' && r.name === 'higiene'))
    return { state: 'unread', action: retry };
  if (!next && !rows.some((r) => r.kind === 'hygiene'))
    return { state: 'empty', message: 'Nenhum secret encontrado' };
  return undefined;
}
