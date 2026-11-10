import { useState } from 'react';
import { Select } from '../../shared/ui/Select';
import { DataTable } from '../../shared/ui/DataTable';
import { exactNumber, formatDate } from '../radio/network-format';
import { DeviceHealthSection } from './device-health';
export { DeviceHealthSection } from './device-health';
import type { PPReadResult } from '../../platform/wails/presentation';
export function DeviceOperationalTable({
  result,
  group,
}: {
  result: PPReadResult;
  group?: string;
}) {
  const groups: Record<string, string> = {
    service: 'services',
    package: 'packages',
    interface: 'interfaces',
    pool: 'pools',
    port: 'ports',
    auth: 'auth',
    fasttrack: 'fasttrack',
  };
  const rows = result.rows.filter((r) =>
    group
      ? groups[r.kind] === group ||
        (r.kind === 'operational' && r.event === group)
      : r.kind === 'operational' ||
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
              <td>
                {group
                  ? OP_LABELS[group]
                  : r.kind || 'Não informado nesta leitura'}
              </td>
              <td>
                {r.kind === 'service' && r.disabled !== undefined
                  ? r.disabled
                    ? 'Desativado'
                    : 'Ativado'
                  : r.used !== undefined
                    ? exactNumber(r.used)
                    : r.size !== undefined
                      ? exactNumber(r.size)
                      : r.event || 'Não informado nesta leitura'}
              </td>
              <td>
                {r.source_at
                  ? `${formatDate(r.source_at)} (${r.value || 'sem detalhe'})`
                  : r.kind === 'availability' && r.name === 'auth'
                    ? `${r.at || 'Horário indisponível'} a ${r.value || 'Horário indisponível'}`
                    : r.value ||
                      (r.at ? formatDate(r.at) : 'Contador acumulado')}
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
  const [group, setGroup] = useState('services');
  const health = result.rows.find((r) => r.kind === 'resource')?.health;
  const authCount = result.rows.find(
    (r) => r.kind === 'availability' && r.name === 'auth',
  )?.used;
  return (
    <section aria-label="Resultado operacional">
      <div className="bar">
        <label>
          Grupo{' '}
          <Select
            aria-label="Grupo"
            value={group}
            onChange={(e) => setGroup(e.target.value)}
          >
            {Object.entries(OP_LABELS).map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </Select>
        </label>
        <details className="det">
          <summary>Detalhes da leitura</summary>
          {health && <DeviceHealthSection health={health} />}
          <p>
            Somente leitura. A auditoria de serviços não verifica “Available
            From” nem altera a configuração.
          </p>
          {Object.entries(OP_LABELS).map(([id, label]) => (
            <p key={id}>
              {label}: {availabilityText(result.availability?.[id])}.
            </p>
          ))}
          <p>
            Contadores de interfaces são acumulados; ausência não é zero. Pools
            em 80% ou mais são destacados apenas nesta leitura. Falhas de
            autenticação representam o buffer examinado, com os horários
            informados pelo roteador; não são uma janela de 24 h.
          </p>
          {authCount !== undefined &&
            result.availability?.auth === 'complete' && (
              <p>
                {authCount === 0
                  ? 'Nenhuma falha de autenticação reconhecida no buffer examinado.'
                  : `${authCount} falhas de autenticação reconhecidas no buffer examinado.`}
              </p>
            )}
        </details>
      </div>
      {result.availability?.[group] !== 'complete' && (
        <p role="status">
          {OP_LABELS[group]}: {availabilityText(result.availability?.[group])}.
        </p>
      )}
      <DeviceOperationalTable result={result} group={group} />
    </section>
  );
}
