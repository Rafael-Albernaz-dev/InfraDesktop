import { DataTable } from './ui/DataTable';
import type { DeviceMonitoringPoint } from './presentation';
import { formatDate } from './network-format';
export function SessionPointsTable({
  points,
}: {
  points: DeviceMonitoringPoint[];
}) {
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Pontos de sessões online"
    >
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Período</th>
            <th>Online mínimo / máximo / último</th>
            <th>Origem das sessões</th>
            <th>Qualidade</th>
          </tr>
        </thead>
        <tbody>
          {points.map((p, i) => {
            const qualityLabels = [
              p.gap ? 'Lacuna de continuidade' : '',
              p.drop ? 'Queda brusca observada' : '',
              !p.gap && !p.drop ? 'Leitura compatível' : '',
            ]
              .filter(Boolean)
              .join(' · ');
            return (
              <tr key={i}>
                <td>
                  {formatDate(p.at)} ·{' '}
                  {p.resolution === 3600 ? 'hora' : 'minuto'}
                </td>
                <td>
                  {p.online_min ?? 'Não informado nesta leitura'} /{' '}
                  {p.online_max ?? 'Não informado nesta leitura'} /{' '}
                  {p.online ?? 'Não informado nesta leitura'}
                </td>
                <td>
                  {p.session_at
                    ? formatDate(p.session_at)
                    : 'Sem leitura de sessões neste ponto'}
                </td>
                <td>
                  {p.online === undefined
                    ? 'Sem leitura de sessões'
                    : qualityLabels}
                </td>
              </tr>
            );
          })}
        </tbody>
      </DataTable>
    </div>
  );
}
