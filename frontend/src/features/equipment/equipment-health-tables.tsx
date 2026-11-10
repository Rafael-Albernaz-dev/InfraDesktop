import { displayPort } from './equipment-port-format';
import { exactNumber, exactRate, formatDate } from './network-format';
import type { PPReadRow } from '../../platform/wails/presentation';
import { DataTable } from '../../shared/ui/DataTable';
export function EquipmentPorts({
  ports,
  compact = false,
}: {
  ports: PPReadRow[];
  compact?: boolean;
}) {
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
            {!compact && <th>Leitura</th>}
          </tr>
        </thead>
        <tbody>
          {ports.map((r) => (
            <tr key={r.name}>
              <td>{r.name}</td>
              <td title={exactRate(r.port?.speed_bps)}>
                {displayPort(r.port)}
                {r.port?.running &&
                ((r.port.speed_bps !== undefined &&
                  r.port.speed_bps <= 100000000) ||
                  r.port.full_duplex === false)
                  ? ' · atenção'
                  : ''}
              </td>
              <td>{exactNumber(r.port?.link_downs)}</td>
              {!compact && <td title={r.at}>{formatDate(r.at)}</td>}
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
