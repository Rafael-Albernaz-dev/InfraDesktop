import type { RadioClient, RadioMetric } from './radio-api';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { RadioReading } from './radio-api';
import { DataTable } from './ui/DataTable';
import { formatMetric } from './network-format';
type Ready = RadioEquipmentState & { reading: RadioReading };
export function radioMetric(metrics: RadioMetric[], name: string) {
  return metrics.find((m) => m.name === name && m.quality !== 'unavailable');
}
export function radioRatePair(metrics: RadioMetric[]) {
  const tx = radioMetric(metrics, 'rate_tx'),
    rx = radioMetric(metrics, 'rate_rx');
  if (!tx || !rx || tx.unit !== rx.unit) return 'Não informado nesta leitura';
  return `${formatMetric(tx.value, tx.unit).replace(' ' + tx.unit, '')}/${formatMetric(rx.value, rx.unit)}`;
}
function RadioClientRow({ client }: { client: RadioClient }) {
  const value = (name: string) => {
    const m = radioMetric(client.metrics, name);
    return m
      ? formatMetric(m.value, m.unit).replace(' %', '%')
      : 'Não informado nesta leitura';
  };
  return (
    <tr>
      <td title="Nome observado; vínculo com o cadastro ainda não verificado (L-BASE-2)">
        {client.name || client.mac}
      </td>
      <td>{value('signal')}</td>
      <td>{value('ccq')}</td>
      <td>{radioRatePair(client.metrics)}</td>
      <td>{value('occupancy')}</td>
    </tr>
  );
}
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
