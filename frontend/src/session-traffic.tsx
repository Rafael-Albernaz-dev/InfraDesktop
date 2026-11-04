import { Chart } from './ui/Chart';
import type { PPReadRow } from './presentation';
import {
  exactNumber,
  exactRate,
  exactVolume,
  formatDuration,
  formatRate,
  formatVolume,
} from './network-format';
import { RateValue } from './pppoe-read-views';
const elapsed = (seconds: number) =>
  `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')}:${Math.floor(seconds % 60)
    .toString()
    .padStart(2, '0')}`;
export function SessionTraffic({
  row,
  live,
}: {
  row: PPReadRow;
  live: boolean;
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
        formatX={elapsed}
        xSteps={[2, 5, 10, 20, 30, 60, 120, 300]}
        live={live}
        label={'Gráfico de tráfego de ' + row.login}
      />
      <p>
        Média da observação: download <RateValue rate={t.average_download} /> /
        upload <RateValue rate={t.average_upload} /> · pico amostrado: download{' '}
        <RateValue rate={t.peak_download} /> / upload{' '}
        <RateValue rate={t.peak_upload} />
      </p>
      <p>
        Total {t.gaps ? 'parcial ' : ''}da observação:{' '}
        <span
          title={`${exactVolume(t.download_bytes)} download / ${exactVolume(t.upload_bytes)} upload`}
        >
          {t.covered_ns > 0 && !t.overflow
            ? `${formatVolume(t.download_bytes)} download / ${formatVolume(t.upload_bytes)} upload`
            : 'Indisponível'}
        </span>{' '}
        · tempo coberto{' '}
        <span title={exactNumber(t.covered_ns / 1e9, 's')}>
          {formatDuration(t.covered_ns / 1e9)}
        </span>
        {t.gaps
          ? ` · ${t.gaps} lacuna(s); sem estimar os intervalos ausentes.`
          : ''}
      </p>
      {t.overflow && <p role="alert">Limite numérico: resumo indisponível.</p>}
    </section>
  );
}
