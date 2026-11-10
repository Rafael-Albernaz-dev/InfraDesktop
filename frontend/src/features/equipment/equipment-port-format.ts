import { portLabel } from './network-format';
import type { PPReadRow } from '../../platform/wails/presentation';

export const displayPort = (port?: PPReadRow['port']) =>
  portLabel(port)
    .replace(/^DOWN$/, 'Parada')
    .replace(/(\d)([KMG]bps)/g, '$1 $2')
    .replace(/-(full|half)/, ' · $1');
