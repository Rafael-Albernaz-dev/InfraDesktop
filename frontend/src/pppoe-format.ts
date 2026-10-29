import type { PPInterval } from './presentation';
import { formatDuration } from './network-format';

export function formatPPUptime(value: PPInterval, compact = false): string {
  return value.known && value.max >= value.min
    ? formatDuration(value.min / 1e9, compact)
    : 'Indisponível';
}
