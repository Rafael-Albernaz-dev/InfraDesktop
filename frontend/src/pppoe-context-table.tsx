import type { PPGroup } from './presentation';
import { formatMetric } from './network-format';
import { DataTable } from './ui/DataTable';
export function ContextCountsPolicyInputs(p: {
  dropThreshold: string;
  setDropThreshold: (v: string) => void;
  minimumPeak: string;
  setMinimumPeak: (v: string) => void;
  validPolicy: boolean;
}) {
  return (
    <>
      <label>
        Queda local (%)
        <input
          type="number"
          min="1"
          max="100"
          aria-label="Queda local"
          size={4}
          className="small-number"
          value={p.dropThreshold}
          onChange={(e) => p.setDropThreshold(e.target.value)}
        />
      </label>
      <label>
        Pico mínimo de sessões
        <input
          type="number"
          min="1"
          max="2048"
          aria-label="Pico mínimo de sessões"
          size={4}
          className="small-number"
          value={p.minimumPeak}
          onChange={(e) => p.setMinimumPeak(e.target.value)}
        />
      </label>
      {!p.validPolicy && (
        <p role="status">
          Informe queda entre 1 e 100% e pico mínimo entre 1 e 2048.
        </p>
      )}
    </>
  );
}
export function ContextGroupsList({
  groups,
  validPolicy,
  minimum,
  threshold,
}: {
  groups: PPGroup[];
  validPolicy: boolean;
  minimum: number;
  threshold: number;
}) {
  return (
    <DataTable>
      <thead>
        <tr>
          <th>VLAN</th>
          <th>Serviço</th>
          <th>Sessões</th>
          <th>Pico 7 dias</th>
        </tr>
      </thead>
      <tbody>
        {groups.map((g, i) => (
          <tr key={i}>
            <td>{g.vlan ?? 'Não informado nesta leitura'}</td>
            <td>{g.service || 'Não informado nesta leitura'}</td>
            <td>
              {g.count}
              {validPolicy &&
                g.peak_7d !== undefined &&
                g.peak_7d >= minimum &&
                ((g.peak_7d - g.count) * 100) / g.peak_7d >= threshold && (
                  <strong> · Queda local detectada</strong>
                )}
              {g.history_limited && (
                <span> · histórico podado; pico entre amostras retidas</span>
              )}
            </td>
            <td
              title={
                g.peak_7d
                  ? `Δ ${formatMetric((Math.abs(g.count - g.peak_7d) * 100) / g.peak_7d, '%')}`
                  : ''
              }
            >
              {g.peak_7d ?? 'Não informado nesta leitura'}
            </td>
          </tr>
        ))}
      </tbody>
    </DataTable>
  );
}
