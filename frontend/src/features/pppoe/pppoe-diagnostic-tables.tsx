import type { PDRow } from '../../platform/wails/presentation';
import { DataTable } from '../../shared/ui/DataTable';
import { KeyValues } from '../../shared/ui/KeyValues';
import {
  exactNumber,
  formatDate,
  formatLatencyRange,
} from '../radio/network-format';
const flag = (value?: boolean) =>
  value === undefined ? 'Não informado nesta leitura' : value ? 'Sim' : 'Não';
function catalogRowDetails(row: PDRow) {
  return [
    `Em execução: ${flag(row.running)}`,
    `Dinâmica: ${flag(row.dynamic)}`,
    `Service tag: ${flag(row.service_tag)}`,
    `Relação: ${row.relation || 'Não informado nesta leitura'}`,
  ].join(' · ');
}
export function CatalogTable({ rows }: { rows: PDRow[] }) {
  return (
    <DataTable>
      <thead>
        <tr>
          {[
            'Nome',
            'Tipo',
            'Interface pai',
            'VLAN / tag',
            'Aceita sem tag',
            'Desativado',
          ].map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows
          .filter((r) => !r.not_seen_since)
          .map((r) => (
            <tr key={r.id}>
              <td title={catalogRowDetails(r)}>
                {r.name || 'Não informado nesta leitura'}
              </td>
              <td>
                {r.kind === 'service'
                  ? 'Serviço PPPoE'
                  : r.kind === 'vlan'
                    ? 'VLAN'
                    : r.type || r.kind}
              </td>
              <td>{r.parent || 'Não informado nesta leitura'}</td>
              <td>
                {r.kind === 'vlan'
                  ? (r.vlan_id ?? 'Não informado nesta leitura')
                  : r.kind === 'service'
                    ? r.range === '?'
                      ? 'Não informado nesta leitura'
                      : r.range || 'Sem faixa configurada'
                    : '—'}
              </td>
              <td>{r.kind === 'service' ? flag(r.accept_untagged) : '—'}</td>
              <td>{flag(r.disabled)}</td>
            </tr>
          ))}
      </tbody>
    </DataTable>
  );
}
export function AbsentInterfacesTable({ rows }: { rows: PDRow[] }) {
  return (
    <DataTable>
      <thead>
        <tr>
          <th>Interface</th>
          <th>Não vista desde</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id}>
            <td>{r.name}</td>
            <td title={r.not_seen_since}>{formatDate(r.not_seen_since)}</td>
          </tr>
        ))}
      </tbody>
    </DataTable>
  );
}
export function RadiusTable({ rows }: { rows: PDRow[] }) {
  return (
    <DataTable>
      <thead>
        <tr>
          {[
            'Servidor',
            'Serviços',
            'Protocolo',
            'Autenticação',
            'Accounting',
            'Desativado',
            'RTT',
            'Respostas inválidas',
          ].map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows
          .filter((r) => r.kind === 'radius')
          .map((r) => (
            <tr key={r.id}>
              <td>{r.address}</td>
              <td>{r.services}</td>
              <td>
                {r.protocol === '?'
                  ? 'Não informado nesta leitura'
                  : r.protocol}
              </td>
              <td>porta {r.auth_port}</td>
              <td>porta {r.accounting_port}</td>
              <td>{flag(r.disabled)}</td>
              <td>
                {r.counters?.last_rtt.known
                  ? formatLatencyRange(r.counters.last_rtt)
                  : 'Não informado nesta leitura'}
              </td>
              <td>{exactNumber(r.counters?.bad_replies)}</td>
            </tr>
          ))}
      </tbody>
    </DataTable>
  );
}
export function RadiusCounterDetails({ rows }: { rows: PDRow[] }) {
  return (
    <>
      {rows
        .filter((r) => r.counters)
        .map((r) => (
          <section key={r.id} aria-label={`Contadores de ${r.address}`}>
            <h3>{r.address}</h3>
            <KeyValues
              rows={[
                ['Pendentes', exactNumber(r.counters!.pending)],
                ['Solicitações', exactNumber(r.counters!.requests)],
                ['Aceitas', exactNumber(r.counters!.accepts)],
                ['Rejeitadas', exactNumber(r.counters!.rejects)],
                ['Reenvios', exactNumber(r.counters!.resends)],
                ['Timeouts', exactNumber(r.counters!.timeouts)],
                ['Respostas inválidas', exactNumber(r.counters!.bad_replies)],
                [
                  'RTT da última solicitação',
                  r.counters!.last_rtt.known
                    ? formatLatencyRange(r.counters!.last_rtt)
                    : 'Não informado nesta leitura',
                ],
              ]}
            />
          </section>
        ))}
    </>
  );
}
export function AAAInfo({ row, at }: { row?: PDRow; at: string }) {
  return (
    <KeyValues
      rows={[
        ['Usa RADIUS', flag(row?.use_radius)],
        ['Accounting', flag(row?.accounting)],
        ['Última leitura boa', formatDate(at)],
      ]}
    />
  );
}
