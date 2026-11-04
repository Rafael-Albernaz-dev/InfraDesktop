import { radioMetric } from './radio-table-format';
export { radioMetric, radioRatePair } from './radio-table-format';
import { RadioClientRow } from './radio-client-row';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { RadioReading } from './radio-api';
import { DataTable } from './ui/DataTable';
import { formatMetric } from './network-format';
type Ready = RadioEquipmentState & { reading: RadioReading };
export function RadioClients({ state }: { state: Ready }) {
  if (!state.reading.clients.length) return <p>Nenhum cliente encontrado</p>;
  return (
    <DataTable className="radio-clients-table">
      <thead>
        <tr>
          <th>CPE</th>
          <th>Sinal</th>
          <th>CCQ</th>
          <th>Taxas (TX/RX)</th>
          <th>Ocupação</th>
        </tr>
      </thead>
      <tbody>
        {state.reading.clients.map((client) => (
          <RadioClientRow
            key={client.interface + '|' + client.mac}
            client={client}
            name={
              state.devices?.find((d) => d.id === client.cpe_id)?.name ||
              client.name ||
              client.mac
            }
            open={state.openEquipment}
          />
        ))}
      </tbody>
    </DataTable>
  );
}
export function RadioInterfaces({ state }: { state: Ready }) {
  return (
    <DataTable>
      <thead>
        <tr>
          <th>Interface</th>
          <th>BSSID</th>
          <th>Canal</th>
          <th>Largura</th>
          <th>Ruído</th>
        </tr>
      </thead>
      <tbody>
        {state.reading.interfaces.map((iface) => (
          <tr key={iface.name}>
            <td>
              {iface.name}
              {iface.master && ` (AP virtual de ${iface.master})`}
            </td>
            <td>{iface.bssid}</td>
            <td>{iface.channel || 'Não informado nesta leitura'}</td>
            <td>{iface.configured_width || 'Não informado nesta leitura'}</td>
            <td>
              {radioMetric(iface.metrics, 'noise')
                ? formatMetric(
                    radioMetric(iface.metrics, 'noise')!.value,
                    'dBm',
                  )
                : 'Não informado nesta leitura'}
            </td>
          </tr>
        ))}
      </tbody>
    </DataTable>
  );
}
