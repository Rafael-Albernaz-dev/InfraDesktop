import type { InventoryState } from './inventory-selection-state';
import { RoleOptions } from './equipment-roles';
import { Button } from './ui/Button';
import { Checkbox } from './ui/Checkbox';
import { Select } from './ui/Select';
import { InventoryColumnsMenu } from './inventory-preferences';

export function InventoryToolbar({
  p,
  locked,
  columns,
  saveColumns,
  busy,
  onAdd,
}: {
  p: InventoryState;
  locked: boolean;
  columns: string[];
  saveColumns: (v: string[]) => void;
  busy: boolean;
  onAdd?: () => void;
}) {
  const change = (action: () => void) => {
    action();
    p.setCursor(0);
    p.setSelectedRows({});
  };
  const clear = () =>
    change(() => {
      p.setSearch('');
      p.setStateFilter('');
      p.setInactive(false);
      p.setRoleFilter('');
      p.setBlockFilter('');
      p.setModelFilter('');
    });
  const filtered =
    p.stateFilter ||
    p.search ||
    p.inactive ||
    p.roleFilter ||
    p.blockFilter ||
    p.modelFilter;
  return (
    <div className="toolbar inventory-toolbar">
      {searchStateRole(p, change)}
      {blockModelActivity(p, change)}
      {filtered && <Button onClick={clear}>Limpar filtros</Button>}
      <span className="grow" />
      <InventoryColumnsMenu value={columns} save={saveColumns} busy={busy} />
      <Button
        kind="primary"
        data-add-trigger
        disabled={locked || !onAdd}
        onClick={onAdd}
      >
        Adicionar equipamento
      </Button>
    </div>
  );
}

type FilterChange = (action: () => void) => void;
function searchStateRole(p: InventoryState, change: FilterChange) {
  return (
    <>
      <input
        aria-label="Buscar"
        placeholder="Buscar nome ou IP"
        value={p.search}
        onChange={(e) => change(() => p.setSearch(e.target.value))}
      />
      <Select
        aria-label="Estado"
        value={p.stateFilter}
        onChange={(e) => change(() => p.setStateFilter(e.target.value))}
      >
        <option value="">Todos</option>
        <option value="ONLINE">ONLINE</option>
        <option value="OFFLINE">OFFLINE</option>
        <option value="unknown">Sem estado</option>
      </Select>
      <Select
        aria-label="Papel"
        value={p.roleFilter}
        onChange={(e) => change(() => p.setRoleFilter(e.target.value))}
      >
        <option value="">Todos os papéis</option>
        <RoleOptions />
      </Select>
    </>
  );
}
function blockModelActivity(p: InventoryState, change: FilterChange) {
  return (
    <>
      <Select
        aria-label="Bloco"
        value={p.blockFilter}
        onChange={(e) => change(() => p.setBlockFilter(e.target.value))}
      >
        <option value="">Todos os blocos</option>
        <option value="none">Sem bloco</option>
        {p.blocks.map((block) => (
          <option key={block} value={`b:${block}`}>
            {block}
          </option>
        ))}
      </Select>
      <Select
        aria-label="Modelo"
        value={p.modelFilter}
        onChange={(e) => change(() => p.setModelFilter(e.target.value))}
      >
        <option value="">Todos os modelos</option>
        {p.models.map((model) => (
          <option key={model} value={model}>
            {model}
          </option>
        ))}
      </Select>
      <label>
        <Checkbox
          checked={p.inactive}
          onChange={(e) => change(() => p.setInactive(e.target.checked))}
        />{' '}
        Mostrar inativos
      </label>
    </>
  );
}
