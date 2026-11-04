import { equipmentType } from './equipment-layout';
import type { Device } from './equipment-api';
import { latestSavedReading } from './equipment-reading';
import { formatDate, formatDuration, formatMetric } from './network-format';
import type { useEquipmentState } from './equipment-state';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';
import { Pager } from './ui/Pager';
import { Select } from './ui/Select';

type Props = ReturnType<typeof useEquipmentState> & {
  connected: boolean;
  primaryInHeader: boolean;
  locked: boolean;
};
function InventoryRow({
  device: d,
  now,
  open,
}: {
  device: Device;
  now: number;
  open: (device: Device) => void;
}) {
  const saved = latestSavedReading(d);
  return (
    <tr
      data-device-id={d.id}
      className="click"
      tabIndex={0}
      onClick={() => open(d)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          open(d);
        }
      }}
    >
      <td>
        {d.name}
        {!d.active && <span className="muted"> (inativo)</span>}
      </td>
      <td>{d.address}</td>
      <td>{saved.version || 'Não informado nesta leitura'}</td>
      <td>{equipmentType(d.role)}</td>
      <td title={saved.exactUptime}>{saved.at ? saved.uptime : '—'}</td>
      <td>
        {saved.at && saved.cpu !== undefined
          ? formatMetric(saved.cpu, '%')
          : '—'}
      </td>
      <td>SSH</td>
      <td title={saved.at ? formatDate(saved.at) : ''}>
        {saved.at
          ? `há ${formatDuration((now - Date.parse(saved.at)) / 1000)}`
          : 'Ainda sem leitura'}
      </td>
    </tr>
  );
}
export function EquipmentInventoryPane(p: Props) {
  const clear = () => {
    p.setSearch('');
    p.setInactive(false);
    p.setCursor(0);
  };
  return (
    <section hidden={p.detailOpen}>
      <h1>Equipamentos</h1>
      <div className="toolbar">
        <input
          aria-label="Buscar"
          placeholder="Buscar"
          value={p.search}
          onChange={(e) => {
            p.setSearch(e.target.value);
            p.setCursor(0);
          }}
        />
        <Select
          aria-label="Estado"
          disabled
          title="Estado de conectividade: Ainda não existe (L-BASE-2)"
        >
          <option>Todos</option>
        </Select>
        <label>
          <input
            type="checkbox"
            checked={p.inactive}
            onChange={(e) => {
              p.setInactive(e.target.checked);
              p.setCursor(0);
            }}
          />{' '}
          Mostrar inativos
        </label>
        <span className="grow" />
        <Button
          kind="primary"
          disabled={p.locked}
          onClick={() => p.setForm('new')}
        >
          Adicionar equipamento
        </Button>
      </div>
      {!p.ready && !p.listError && (
        <p role="status">Carregando equipamentos…</p>
      )}
      <DataTable aria-label="Lista de equipamentos">
        <thead>
          <tr>
            {[
              'Equipamento',
              'Endereço',
              'Sistema',
              'Papel',
              'Uptime',
              'CPU',
              'Consulta',
              'Última leitura',
            ].map((label) => (
              <th key={label}>{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {p.devices.map((device) => (
            <InventoryRow
              key={device.id}
              device={device}
              now={p.displayNow}
              open={p.openDevice}
            />
          ))}
          {p.ready && !p.listError && !p.devices.length && (
            <tr>
              <td colSpan={8}>
                {p.search || p.inactive ? (
                  <>
                    Nenhum equipamento encontrado{' '}
                    <Button onClick={clear}>Limpar filtros</Button>
                  </>
                ) : (
                  'Nenhum equipamento cadastrado'
                )}
              </td>
            </tr>
          )}
        </tbody>
      </DataTable>
      <Pager
        page={p.cursor ? 2 : 1}
        total={p.next ? undefined : p.cursor ? 2 : 1}
        hasNext={Boolean(p.next)}
        first={() => p.setCursor(0)}
        next={() => p.setCursor(p.next)}
      />
    </section>
  );
}
