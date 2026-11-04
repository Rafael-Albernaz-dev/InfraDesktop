import {
  exactNumber,
  formatDate,
  formatLatency,
  formatMetric,
} from './network-format';
import type { PDRow } from './presentation';

export function PingResultDetails(p: {
  result: NonNullable<PDRow['ping']>;
  metaDate: string;
}) {
  const { result, metaDate } = p;
  const baselineDiff =
    result.baseline_median_ns !== undefined &&
    result.baseline_median_ns > 0 &&
    result.avg_ns !== undefined
      ? ` · RTT médio ${result.avg_ns >= result.baseline_median_ns ? '+' : '−'}` +
        `${formatMetric((Math.abs(result.avg_ns - result.baseline_median_ns) * 100) / result.baseline_median_ns, '%')}`
      : '';
  const minText = formatLatency(result.min_ns);
  const avgText = formatLatency(result.avg_ns);
  const maxText = formatLatency(result.max_ns);
  const jitterText = formatLatency(result.jitter_ns);
  const baseMedText = formatLatency(result.baseline_median_ns);
  const samplesText =
    result.baseline_samples !== undefined
      ? `${result.baseline_samples}/10 amostras anteriores disponíveis.`
      : 'Histórico não registrado.';
  return (
    <>
      <p>
        Origem: {result.context} · {formatDate(metaDate)}
      </p>
      <p>
        Enviados: {result.sent} · Recebidos: {result.received} · Perda:{' '}
        {formatMetric(result.loss_percent, '%')}
      </p>
      <p>
        RTT mínimo:{' '}
        <span title={exactNumber(result.min_ns, 'ns')}>{minText}</span> · médio:{' '}
        <span title={exactNumber(result.avg_ns, 'ns')}>{avgText}</span> ·
        máximo: <span title={exactNumber(result.max_ns, 'ns')}>{maxText}</span>
      </p>
      <p>
        Amostra de 3 probes · jitter:{' '}
        <span title={exactNumber(result.jitter_ns, 'ns')}>{jitterText}</span>.
      </p>
      <p>
        Mediana dos 10 pings anteriores bem-sucedidos deste alvo e sessão:{' '}
        <span title={exactNumber(result.baseline_median_ns, 'ns')}>
          {baseMedText}
        </span>
        {baselineDiff}. {samplesText} Histórico em memória desde o início do
        serviço; perdas não entram na baseline.
      </p>
      <ol>
        {result.probes.map((probe) => (
          <li key={probe.sequence} value={probe.sequence + 1}>
            {probe.status || 'Resposta ICMP'}
            {probe.rtt_ns !== undefined
              ? ` · ${formatLatency(probe.rtt_ns)}`
              : ''}
          </li>
        ))}
      </ol>
    </>
  );
}
