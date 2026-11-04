import { DataTable } from './ui/DataTable';
import type { PPReadRow } from './presentation';
import {
  exactNumber,
  exactRate,
  formatDate,
  portLabel,
} from './network-format';
export function EquipmentPorts({ ports }: { ports: PPReadRow[] }) {
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Portas físicas negociadas"
    >
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Porta</th>
            <th>Velocidade / duplex</th>
            <th>Quedas acumuladas</th>
            <th>Leitura</th>
          </tr>
        </thead>
        <tbody>
          {ports.map((r) => (
            <tr key={r.name}>
              <td>{r.name}</td>
              <td title={exactRate(r.port?.speed_bps)}>
                {portLabel(r.port)}
                {r.port?.running &&
                ((r.port.speed_bps !== undefined &&
                  r.port.speed_bps <= 100000000) ||
                  r.port.full_duplex === false)
                  ? ' · atenção'
                  : ''}
              </td>
              <td>{exactNumber(r.port?.link_downs)}</td>
              <td title={r.at}>{formatDate(r.at)}</td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
