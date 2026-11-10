import { Chart } from '../../shared/ui/Chart';
import type { DeviceMonitoringPoint } from '../../platform/wails/presentation';
import { SessionPointsTable } from './monitoring-points';

function SessionCurve({ points }: { points: DeviceMonitoringPoint[] }) {
  const ordered = [...points].reverse();
  const values = ordered.map((p, i) => {
    const prev = ordered[i - 1];
    const missingInterval =
      prev &&
      Date.parse(p.at) - Date.parse(prev.at) >
        Math.max(p.resolution, prev.resolution) * 1000;
    return p.gap || missingInterval ? null : (p.online ?? null);
  });
  return (
    <Chart
      x={ordered.map((p) => Date.parse(p.at) / 1000)}
      series={[{ label: 'Sessões online', token: '--chart-down', values }]}
      format={(value) => (value == null ? '—' : String(Math.round(value)))}
      formatX={(value) =>
        new Date(value * 1000).toLocaleTimeString('pt-BR', {
          hour: '2-digit',
          minute: '2-digit',
        })
      }
      xSteps={[60, 300, 900, 3600, 10800, 21600, 43200, 86400]}
      markers={ordered.flatMap((p, i) => (p.drop ? [i] : []))}
      label="Curva de sessões online observadas"
    />
  );
}

export function SessionMonitoringCurve({
  points,
}: {
  points: DeviceMonitoringPoint[];
}) {
  const valid = points.filter((p) => !p.gap && p.online !== undefined);
  const values = valid.map((p) => p.online!);
  return (
    <>
      <SessionCurve points={points} />
      <p className="muted">
        Online mínimo / máximo / último:{' '}
        {values.length ? Math.min(...values) : '—'} /{' '}
        {values.length ? Math.max(...values) : '—'} / {values[0] ?? '—'} ·
        lacunas não são zero. Ponto vermelho: queda brusca.
      </p>
      <details className="det">
        <summary>Pontos da curva</summary>
        <SessionPointsTable points={points} />
      </details>
    </>
  );
}
