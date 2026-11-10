import type { PPReadPanelProps } from './pppoe-read-props';

export function readHeading(p: PPReadPanelProps) {
  if (p.traffic) return 'Tráfego';
  if (p.evidence === 'log') return 'Log PPP';
  if (p.evidence === 'secret') return 'Evidência do secret';
  if (p.monitoring) return 'Leitura operacional';
  if (p.health) return healthHeading(p.healthView);
  if (p.bandwidthOnly) return 'Banda';
  if (p.allowTop && !p.rows.length) return 'Top consumo';
  return 'Banda por sessão';
}
function healthHeading(view?: 'pools' | 'uplinks') {
  if (view === 'uplinks') return 'Interfaces estáticas';
  if (view === 'pools') return 'Pools IPv4';
  return 'Pools e interfaces estáticas';
}

export function hasReadTarget(p: PPReadPanelProps) {
  return Boolean(
    p.rows.length || p.allowTop || p.logRow || p.health || p.monitoring,
  );
}
