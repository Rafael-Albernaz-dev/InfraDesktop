import type { RadioClient } from './radio-api';
import { formatMetric } from './network-format';
import { radioMetric, radioRatePair } from './radio-table-format';
export function RadioClientRow({
  client,
  name,
  open,
}: {
  client: RadioClient;
  name: string;
  open?: (id: string) => void;
}) {
  const value = (field: string) => {
    const metric = radioMetric(client.metrics, field);
    return metric
      ? formatMetric(metric.value, metric.unit).replace(' %', '%')
      : 'Não informado nesta leitura';
  };
  const linked = Boolean(client.cpe_id && open);
  const navigate = () => {
    if (client.cpe_id) open?.(client.cpe_id);
  };
  return (
    <tr
      className={linked ? 'click' : undefined}
      tabIndex={linked ? 0 : undefined}
      onClick={linked ? navigate : undefined}
      onKeyDown={(event) => {
        if (
          linked &&
          event.target === event.currentTarget &&
          (event.key === 'Enter' || event.key === ' ')
        ) {
          event.preventDefault();
          navigate();
        }
      }}
    >
      <td
        title={
          client.cpe_id
            ? 'Vínculo confirmado por MAC, interface e BSSID'
            : 'Nome observado; sem vínculo confirmado'
        }
      >
        <ClientName client={client} name={name} navigate={navigate} />
      </td>
      <td>{value('signal')}</td>
      <td>{value('ccq')}</td>
      <td>{radioRatePair(client.metrics)}</td>
      <td>{value('occupancy')}</td>
    </tr>
  );
}

function ClientName({
  client,
  name,
  navigate,
}: {
  client: RadioClient;
  name: string;
  navigate: () => void;
}) {
  return client.cpe_id ? (
    <a
      href={`#ficha/${client.cpe_id}`}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        navigate();
      }}
    >
      {name}
    </a>
  ) : (
    <>{name}</>
  );
}
