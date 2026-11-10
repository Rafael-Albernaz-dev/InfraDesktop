import { columns, defaultColumns } from './inventory-columns';
import { Button } from '../../shared/ui/Button';
import { Checkbox } from '../../shared/ui/Checkbox';
import { Popover } from '../../shared/ui/Popover';

export function InventoryColumnsMenu({
  value,
  save,
  busy,
}: {
  value: string[];
  save: (columns: string[]) => void;
  busy: boolean;
}) {
  const selected = value.slice(1),
    remaining = columns.filter((c) => !selected.includes(c.id));
  const ordered = selected
    .map((id) => columns.find((c) => c.id === id))
    .filter((c) => c !== undefined);
  const toggle = (id: string) =>
    save([
      'equipment',
      ...(selected.includes(id)
        ? selected.filter((v) => v !== id)
        : [...selected, id]),
    ]);
  const move = (index: number, offset: number) => {
    const next = [...selected],
      target = index + offset;
    [next[index], next[target]] = [next[target], next[index]];
    save(['equipment', ...next]);
  };
  return (
    <Popover label="Colunas">
      <div className="inventory-column-row muted">
        Equipamento sempre aparece
      </div>
      {ordered.map((column, index) => (
        <div className="inventory-column-row" key={column.id}>
          <label>
            <Checkbox
              checked
              disabled={busy}
              onChange={() => toggle(column.id)}
            />{' '}
            {column.label}
          </label>
          <Button
            disabled={busy || index === 0}
            label={`Subir ${column.label}`}
            onClick={() => move(index, -1)}
          >
            ↑
          </Button>
          <Button
            disabled={busy || index === ordered.length - 1}
            label={`Descer ${column.label}`}
            onClick={() => move(index, 1)}
          >
            ↓
          </Button>
        </div>
      ))}
      <hr />
      {remaining.map((column) => (
        <div className="inventory-column-row" key={column.id}>
          <label>
            <Checkbox
              checked={false}
              disabled={busy}
              onChange={() => toggle(column.id)}
            />{' '}
            {column.label}
          </label>
        </div>
      ))}
      <hr />
      <Button disabled={busy} onClick={() => save(defaultColumns)}>
        Restaurar padrão
      </Button>
    </Popover>
  );
}
