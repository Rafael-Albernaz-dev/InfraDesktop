import type { PPInterval } from '../../platform/wails/presentation-pppoe-types';
import { formatDuration } from '../equipment/network-format';

export function formatPPUptime(value: PPInterval, compact = false): string {
  return value.known && value.max >= value.min
    ? formatDuration(value.min / 1e9, compact)
    : 'Não informado nesta leitura';
}
