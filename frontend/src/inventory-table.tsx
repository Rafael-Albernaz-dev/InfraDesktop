import type { Device } from './equipment-api';
import type { InventoryState } from './inventory-selection-state';
import { columnById } from './inventory-columns';
import { Checkbox } from './ui/Checkbox';
import { DataTable } from './ui/DataTable';

export function InventoryTable({
  p,
  visible,
}: {
  p: InventoryState;
  visible: string[];
}) {
  const definitions = visible
    .slice(1)
    .map(columnById)
    .filter((c) => c !== undefined);
  const all =
    p.devices.length > 0 && p.devices.every((d) => p.selectedRows[d.id]);
  const setOne = (d: Device, checked: boolean) =>
    p.setSelectedRows((old) => {
      const next = { ...old };
      if (checked) next[d.id] = d;
      else delete next[d.id];
      return next;
    });
  const setPage = (checked: boolean) =>
    p.setSelectedRows((old) => {
      const next = { ...old };
      for (const d of p.devices) {
        if (checked) next[d.id] = d;
        else delete next[d.id];
      }
      return next;
    });
  return (
    <div
      className={`inventory-scroll ${visible.length === 11 ? 'inventory-standard' : ''}`}
    >
      <DataTable aria-label="Lista de equipamentos">
        <thead>
          <tr>
            <th className="inventory-check">
              <Checkbox
                aria-label="Selecionar todos"
                checked={all}
                onChange={(e) => setPage(e.target.checked)}
              />
            </th>
            <th>Equipamento</th>
            {definitions.map((column) => (
              <th key={column.id} data-column={column.id}>
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {p.devices.map((d) => (
            <tr
              key={d.id}
              data-device-id={d.id}
              className="click"
              tabIndex={0}
              onClick={() => p.openDevice(d)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  p.openDevice(d);
                }
              }}
            >
              <td className="inventory-check">
                <Checkbox
                  aria-label={`Selecionar ${d.name}`}
                  checked={Boolean(p.selectedRows[d.id])}
                  onChange={(e) => setOne(d, e.target.checked)}
                />
              </td>
              <td>
                {d.name}
                {!d.active && <span className="muted"> (inativo)</span>}
              </td>
              {definitions.map((column) => (
                <td key={column.id}>
                  {column.cell({
                    device: d,
                    now: p.displayNow,
                    credentials: p.credentials,
                  })}
                </td>
              ))}
            </tr>
          ))}
          {p.ready && !p.listError && !p.devices.length && (
            <tr>
              <td colSpan={definitions.length + 2}>
                {emptyInventoryMessage(p)}
              </td>
            </tr>
          )}
        </tbody>
      </DataTable>
    </div>
  );
}

function emptyInventoryMessage(p: InventoryState) {
  const filtered =
    p.search ||
    p.inactive ||
    p.roleFilter ||
    p.blockFilter ||
    p.modelFilter ||
    p.stateFilter;
  return filtered
    ? 'Nenhum equipamento encontrado'
    : 'Nenhum equipamento cadastrado';
}
