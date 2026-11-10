import type { PPReadResult } from '../../platform/wails/presentation';
import {
  exactNumber,
  exactVolume,
  formatClockOffset,
  formatDate,
  formatMetric,
  formatVolumePair,
  uptimeTitle,
} from '../equipment/network-format';
import { formatPPUptime } from '../pppoe/pppoe-format';
export function DeviceHealthSection({
  health,
  showFastTrackNotice = true,
}: {
  showFastTrackNotice?: boolean;
  health: NonNullable<PPReadResult['rows'][number]['health']>;
}) {
  const startText = health.started_at
    ? formatDate(health.started_at)
    : 'Instante indisponível';
  const endText = health.at ? formatDate(health.at) : 'Instante indisponível';
  const ntpStatus =
    health.ntp_enabled === undefined
      ? 'Não informado nesta leitura'
      : health.ntp_enabled
        ? 'Ativado'
        : 'Desativado';
  return (
    <>
      <p>
        Recurso observado entre {startText} e {endText}.
      </p>
      <dl className="pppoe-diagnostic-values pppoe-context-values">
        <dt>Regras FastTrack habilitadas</dt>
        <dd>{exactNumber(health.fasttrack_rules)}</dd>
        <dt>CPU</dt>
        <dd>{formatMetric(health.cpu, '%')}</dd>
        <dt>Memória livre / total</dt>
        <dd
          title={`${exactVolume(health.free_memory)} / ${exactVolume(health.total_memory)}`}
        >
          {formatVolumePair(health.free_memory, health.total_memory)}
        </dd>
        <dt>Uptime do equipamento</dt>
        <dd title={uptimeTitle(health.uptime)}>
          {formatPPUptime(health.uptime)}
        </dd>
        <dt>RouterOS</dt>
        <dd>{health.version || 'Não informado nesta leitura'}</dd>
        <dt>Temperatura</dt>
        <dd>
          {health.temperature === undefined
            ? 'Não informado nesta leitura'
            : formatMetric(health.temperature, '°C')}
        </dd>
        <dt>Diferença relógio roteador × PC</dt>
        <dd>
          {health.clock_offset.known
            ? formatClockOffset(health.clock_offset)
            : 'Não informado nesta leitura'}
        </dd>
        <dt>NTP</dt>
        <dd>
          {ntpStatus} · {health.ntp_status || 'Estado indisponível'}
        </dd>
        <dt>Erros no buffer de log</dt>
        <dd>{health.error_count ?? 'Não informado nesta leitura'}</dd>
      </dl>
      {showFastTrackNotice &&
        health.fasttrack_rules !== undefined &&
        health.fasttrack_rules > 0 && (
          <p role="status" className="inventory-notice">
            Sem controle de banda: FastTrack ativo
          </p>
        )}
    </>
  );
}
