import { DataTable } from './ui/DataTable';
import {
  exactNumber,
  exactVolume,
  formatClockOffset,
  formatDate,
  formatMetric,
  formatVolumePair,
  uptimeTitle,
} from './network-format';
import { formatPPUptime } from './pppoe-format';
import type { PPReadResult } from './presentation';
export function DeviceHealthSection({
  health,
}: {
  health: NonNullable<PPReadResult['rows'][number]['health']>;
}) {
  const startText = health.started_at
    ? formatDate(health.started_at)
    : 'Instante indisponível';
  const endText = health.at ? formatDate(health.at) : 'Instante indisponível';
  const ntpStatus =
    health.ntp_enabled === undefined
      ? 'Indisponível'
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
        <dd>{health.version || 'Indisponível'}</dd>
        <dt>Temperatura</dt>
        <dd>
          {health.temperature === undefined
            ? 'Indisponível'
            : formatMetric(health.temperature, '°C')}
        </dd>
        <dt>Diferença relógio roteador × PC</dt>
        <dd>
          {health.clock_offset.known
            ? formatClockOffset(health.clock_offset)
            : 'Indisponível'}
        </dd>
        <dt>NTP</dt>
        <dd>
          {ntpStatus} · {health.ntp_status || 'Estado indisponível'}
        </dd>
        <dt>Erros no buffer de log</dt>
        <dd>{health.error_count ?? 'Indisponível'}</dd>
      </dl>
      {health.fasttrack_rules !== undefined && health.fasttrack_rules > 0 && (
        <p role="status" className="inventory-notice">
          Sem controle de banda: FastTrack ativo
        </p>
      )}
    </>
  );
}
export function DeviceOperationalTable({ result }: { result: PPReadResult }) {
  const rows = result.rows.filter(
    (r) =>
      r.kind === 'operational' ||
      r.kind === 'resource' ||
      r.kind === 'availability',
  );
  return (
    <div className="table-region" tabIndex={0} aria-label="Tabela operacional">
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Item</th>
            <th>Grupo</th>
            <th>Estado / valor / contagem</th>
            <th>Detalhe / instante</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>
              <td>{r.name || 'Item sem nome'}</td>
              <td>{r.kind || 'Indisponível'}</td>
              <td>
                {r.used !== undefined
                  ? exactNumber(r.used)
                  : r.size !== undefined
                    ? exactNumber(r.size)
                    : r.event || 'Indisponível'}
              </td>
              <td>
                {r.source_at
                  ? `${formatDate(r.source_at)} (${r.value || 'sem detalhe'})`
                  : r.kind === 'availability' && r.name === 'auth'
                    ? `${r.at || 'Horário indisponível'} a ${r.value || 'Horário indisponível'}`
                    : r.value || 'Contador acumulado'}
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
const OP_LABELS: Record<string, string> = {
  services: 'Serviços telnet/ftp/www',
  packages: 'Pacotes RouterOS',
  interfaces: 'Erros e quedas de interfaces',
  pools: 'Uso de pools',
  auth: 'Falhas de autenticação PPP',
  fasttrack: 'Regras FastTrack',
  ports: 'Negociação de portas físicas',
};
function availabilityText(avail?: string): string {
  if (avail === 'complete') return 'leitura completa';
  if (avail === 'partial')
    return 'leitura parcial; há formatos não reconhecidos';
  if (avail === 'limited') return 'limite atingido';
  return 'indisponível';
}
export function DeviceOperationalTables({ result }: { result: PPReadResult }) {
  const health = result.rows.find((r) => r.kind === 'resource')?.health;
  const authCount = result.rows.find(
    (r) => r.kind === 'availability' && r.name === 'auth',
  )?.used;
  return (
    <section className="pppoe-diagnostics" aria-label="Resultado operacional">
      {health && <DeviceHealthSection health={health} />}
      <p>
        Somente leitura. A auditoria de serviços não verifica “Available From”
        nem altera a configuração.
      </p>
      {Object.entries(OP_LABELS).map(([group, label]) => (
        <p key={group}>
          {label}: {availabilityText(result.availability?.[group])}.
        </p>
      ))}
      <p>
        Contadores de interfaces são acumulados; ausência não é zero. Pools em
        80% ou mais são destacados apenas nesta leitura. Falhas de autenticação
        representam o buffer examinado, com os horários informados pelo
        roteador; não são uma janela de 24 h.
      </p>
      {authCount !== undefined && result.availability?.auth === 'complete' && (
        <p>
          {authCount === 0
            ? 'Nenhuma falha de autenticação reconhecida no buffer examinado.'
            : `${authCount} falhas de autenticação reconhecidas no buffer examinado.`}
        </p>
      )}
      <DeviceOperationalTable result={result} />
    </section>
  );
}
