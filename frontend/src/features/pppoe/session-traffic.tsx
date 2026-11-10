import { SessionRatePair } from './session-rate-pair';
import { KeyValues } from '../../shared/ui/KeyValues';
import { Chart } from '../../shared/ui/Chart';
import type { PPReadRow } from '../../platform/wails/presentation';
import {
  exactNumber,
  exactRate,
  exactVolume,
  formatDuration,
  formatRate,
  formatVolumePair,
} from '../equipment/network-format';
const elapsed = (seconds: number) =>
  `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')}:${Math.floor(seconds % 60)
    .toString()
    .padStart(2, '0')}`;
export function SessionTraffic({
  row,
  live,
  startedAt,
}: {
  row: PPReadRow;
  live: boolean;
  startedAt?: string;
}) {
  const t = row.traffic;
  if (!t) return <p>Não informado nesta leitura</p>;
  const samples = t.points
    .flatMap((p) => (p.gap ? [] : [p.d, p.u]))
    .filter(
      (value): value is number => value != null && Number.isFinite(value),
    );
  const maximum = samples.length ? Math.max(...samples) : undefined;
  return (
    <section
      aria-label={'Resumo de tráfego de ' + row.login}
      title={
        maximum === undefined
          ? undefined
          : 'Maior amostra: ' + exactRate(maximum)
      }
    >
      <Chart
        x={t.points.map((p) => p.t / 1000)}
        series={[
          {
            label: 'Download',
            token: '--chart-down',
            values: t.points.map((p) => (p.gap ? null : (p.d ?? null))),
          },
          {
            label: 'Upload',
            token: '--chart-up',
            values: t.points.map((p) => (p.gap ? null : (p.u ?? null))),
          },
        ]}
        format={(value) => (value == null ? '—' : formatRate(value))}
        formatX={(seconds) => {
          const start = Date.parse(startedAt || '');
          return Number.isFinite(start)
            ? new Date(start + seconds * 1000).toLocaleTimeString('pt-BR', {
                hour12: false,
              })
            : elapsed(seconds);
        }}
        xSteps={[2, 5, 10, 20, 30, 60, 120, 300]}
        live={live}
        label={'Gráfico de tráfego de ' + row.login}
      />
      <p className="muted">
        Interrupções aparecem como lacunas, nunca como zero.
      </p>
      <KeyValues
        rows={[
          [
            'Média',
            <SessionRatePair
              download={t.average_download}
              upload={t.average_upload}
            />,
          ],
          [
            'Pico amostrado',
            <SessionRatePair
              download={t.peak_download}
              upload={t.peak_upload}
            />,
          ],
          [
            'Total da observação',
            <span
              key="total"
              title={`${exactVolume(t.download_bytes)} download / ${exactVolume(t.upload_bytes)} upload`}
            >
              {t.covered_ns > 0 && !t.overflow
                ? formatVolumePair(t.download_bytes, t.upload_bytes)
                : 'Não informado nesta leitura'}
            </span>,
          ],
          [
            'Tempo coberto',
            <span key="coverage" title={exactNumber(t.covered_ns / 1e9, 's')}>
              {formatDuration(t.covered_ns / 1e9)}
              {t.gaps ? ` · ${t.gaps} lacuna${t.gaps === 1 ? '' : 's'}` : ''}
            </span>,
          ],
        ]}
      />
      {t.overflow && <p role="alert">Limite numérico: resumo indisponível.</p>}
    </section>
  );
}
