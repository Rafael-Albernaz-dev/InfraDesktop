import { exactNumber } from '../radio/network-format';
import type {
  PPReadResult,
  PPReadRow,
} from '../../platform/wails/presentation';
import { DataTable } from '../../shared/ui/DataTable';

import { RateValue } from './pppoe-read-views';

function flag(value?: boolean) {
  if (value === undefined) return 'Não informado nesta leitura';
  return value ? 'Sim' : 'Não';
}
function UplinkRow({ r }: { r: PPReadRow }) {
  return (
    <tr>
      <td>{r.name}</td>
      <td>
        <RateValue rate={r.tx} />
      </td>
      <td>
        <RateValue rate={r.rx} />
      </td>
      <td>{flag(r.running)}</td>
      <td>{flag(r.disabled)}</td>
      <td>{exactNumber(r.rx_errors)}</td>
      <td>{exactNumber(r.tx_errors)}</td>
      <td>{exactNumber(r.rx_drops)}</td>
      <td>{exactNumber(r.tx_drops)}</td>
    </tr>
  );
}
function UplinkTable({ uplinks }: { uplinks: PPReadRow[] }) {
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Interfaces estáticas"
    >
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Interface</th>
            <th title="TX da interface">TX</th>
            <th title="RX da interface">RX</th>
            <th>Em execução</th>
            <th>Desativada</th>
            <th>Erros RX</th>
            <th>Erros TX</th>
            <th>Descartes RX</th>
            <th>Descartes TX</th>
          </tr>
        </thead>
        <tbody>
          {uplinks.map((r, i) => (
            <UplinkRow key={i} r={r} />
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
function PoolBody({ result }: { result: PPReadResult }) {
  const pools = result.rows.filter((r) => r.kind === 'pool');
  if (result.pools_available !== true)
    return <p>Leitura de pools indisponível nesta versão ou permissão.</p>;
  if (!pools.length)
    return (
      <p>
        {result.pool_total === 0
          ? 'Nenhum pool configurado.'
          : 'Pools disponíveis em outra página.'}
      </p>
    );
  return (
    <div className="table-region" tabIndex={0} aria-label="Pools IPv4">
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Pool</th>
            <th>Em uso</th>
            <th>Configurados</th>
          </tr>
        </thead>
        <tbody>
          {pools.map((r, i) => (
            <tr key={i}>
              <td>{r.name}</td>
              <td>{r.used ?? 'Não informado nesta leitura'}</td>
              <td>{r.size ?? 'Não informado nesta leitura'}</td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
function UplinkBody({ result }: { result: PPReadResult }) {
  const uplinks = result.rows.filter((r) => r.kind === 'uplink');
  if (result.uplinks_available !== true)
    return <p>Leitura de interfaces indisponível nesta versão ou permissão.</p>;
  if (!uplinks.length)
    return (
      <p>
        {result.uplink_total === 0
          ? 'Nenhuma interface estática disponível.'
          : 'Interfaces disponíveis em outra página.'}
      </p>
    );
  return <UplinkTable uplinks={uplinks} />;
}
export function PPHealthTables({
  result,
  view,
}: {
  result: PPReadResult;
  view?: 'pools' | 'uplinks';
}) {
  return (
    <>
      {view !== 'uplinks' && (
        <>
          {!view && <h4>Pools IPv4</h4>}
          <PoolBody result={result} />
        </>
      )}
      {view !== 'pools' && (
        <>
          {!view && <h4>Interfaces estáticas</h4>}
          {!view && (
            <p>
              TX e RX são vistos pela interface do concentrador. Tráfego e
              contadores não identificam a causa de uma falha.
            </p>
          )}
          <UplinkBody result={result} />
        </>
      )}
    </>
  );
}
