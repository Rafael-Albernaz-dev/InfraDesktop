import { useEffect, useRef, useState } from 'react';
import { access } from './presentation';
import { columns, defaultColumns } from './inventory-columns';
import { Popover } from './ui/Popover';
import { Checkbox } from './ui/Checkbox';
import { Button } from './ui/Button';

export function useColumnPreferences() {
  const [visible, setVisible] = useState<string[]>(defaultColumns);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const pending = useRef(false);
  const generation = useRef(0);
  useEffect(() => {
    let alive = true;
    const request = generation.current;
    void (async () => {
      try {
        const result = await access.status();
        if (
          alive &&
          request === generation.current &&
          result.code === 'OK' &&
          result.columns?.length
        )
          setVisible(result.columns);
      } catch {
        /* the normal service banner reports unavailable IPC */
      }
    })();
    return () => {
      alive = false;
    };
  }, []);
  async function save(next: string[]) {
    if (pending.current) return;
    pending.current = true;
    generation.current++;
    setBusy(true);
    setError('');
    try {
      const result = await access.setColumns(next);
      if (result.code !== 'OK') throw new Error(result.code);
      setVisible(result.columns || next);
    } catch {
      setError('Não foi possível salvar as colunas. Tente novamente.');
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return { visible, busy, error, save };
}

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
