import { useState } from 'react';
import type { Credential } from './equipment-api';
import type { ImportItem } from './presentation';
import type { useImportState } from './add-import-state';
import { AddEquipmentFields } from './add-equipment-fields';
import { roleGroups } from './equipment-roles';
import { ImportSteps } from './add-import-file';
import { Button } from './ui/Button';
import { Checkbox } from './ui/Checkbox';
import { DataTable } from './ui/DataTable';
import { Dialog } from './ui/Dialog';
import { Select } from './ui/Select';

const labels: Record<ImportItem['status'], string> = {
  ready: 'Pronta',
  pending: 'Pendente',
  duplicate: 'Duplicada',
  conflict: 'Conflito',
  ignored: 'Ignorada',
};
type Filter = 'all' | ImportItem['status'];
type State = ReturnType<typeof useImportState>;
function Correction(p: {
  item: ImportItem;
  state: State;
  credentials: Credential[];
  close: () => void;
}) {
  const [row, setRow] = useState(() => ({
    ...p.item.row,
    role:
      roleGroups
        .flatMap(([, roles]) => roles)
        .find(
          ([id, label]) => id === p.item.row.role || label === p.item.row.role,
        )?.[0] || '',
  }));
  return (
    <Dialog
      portal
      title={`Corrigir linha ${row.line}`}
      close={p.close}
      showClose={false}
    >
      {p.item.reason && (
        <p className="import-status-error" role="alert">
          {p.item.reason}
        </p>
      )}
      <AddEquipmentFields
        row={row}
        correction
        credentials={p.credentials}
        change={(patch) => setRow({ ...row, ...patch })}
      />
      <div className="dialog-actions">
        <Button onClick={p.close}>Cancelar</Button>
        <Button
          kind="primary"
          onClick={() => {
            p.state.update(row.line, row);
            p.close();
          }}
        >
          Salvar linha
        </Button>
      </div>
    </Dialog>
  );
}
function SelectionDialog(p: {
  mode: 'block' | 'credential';
  state: State;
  credentials: Credential[];
  close: () => void;
}) {
  const [value, setValue] = useState('');
  return (
    <Dialog
      portal
      title={p.mode === 'block' ? 'Definir bloco' : 'Definir credencial'}
      close={p.close}
    >
      {p.mode === 'block' ? (
        <label className="field">
          Bloco
          <input value={value} onChange={(e) => setValue(e.target.value)} />
        </label>
      ) : (
        <label className="field">
          Credencial
          <Select
            aria-label="Credencial"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          >
            <option value="">Sem credencial</option>
            {p.credentials.map((c) => (
              <option key={c.id} value={c.alias}>
                {c.alias}
              </option>
            ))}
          </Select>
        </label>
      )}
      <div className="dialog-actions">
        <Button onClick={p.close}>Cancelar</Button>
        <Button
          kind="primary"
          onClick={() => {
            void p.state.review(
              p.state.items.map((item) =>
                p.state.selected.includes(item.row.line)
                  ? { ...item.row, [p.mode]: value }
                  : item.row,
              ),
            );
            p.close();
          }}
        >
          Aplicar
        </Button>
      </div>
    </Dialog>
  );
}
function ImportRowView(p: {
  item: ImportItem;
  state: State;
  correct: () => void;
}) {
  const { item, state: s } = p;
  const row = item.row;
  return (
    <tr>
      <td className="import-check">
        <Checkbox
          aria-label={`Selecionar linha ${row.line}`}
          checked={s.selected.includes(row.line)}
          disabled={item.status === 'ignored'}
          onChange={(e) =>
            s.setSelected(
              e.target.checked
                ? [...s.selected, row.line]
                : s.selected.filter((n) => n !== row.line),
            )
          }
        />
      </td>
      <td>{row.line}</td>
      <td>{row.name}</td>
      <td>{row.address}</td>
      <td>
        {roleGroups
          .flatMap(([, roles]) => roles)
          .find(([id]) => id === row.role)?.[1] || row.role}
      </td>
      <td>{row.block || '—'}</td>
      <td>{row.credential || 'Sem credencial'}</td>
      <td
        className={
          item.status === 'ready' || item.status === 'ignored'
            ? ''
            : 'import-status-error'
        }
      >
        {labels[item.status]}
      </td>
      <td className="import-reason">{item.reason || '—'}</td>
      <td>
        {item.status === 'ignored' ? (
          <Button onClick={() => s.update(row.line, { ignored: false })}>
            Restaurar
          </Button>
        ) : (
          <>
            <Button onClick={p.correct}>Corrigir</Button>{' '}
            <Button onClick={() => s.update(row.line, { ignored: true })}>
              Ignorar
            </Button>
          </>
        )}
      </td>
    </tr>
  );
}
export function ImportPreview(p: {
  state: State;
  credentials: Credential[];
  cancel: () => void;
  replace: () => void;
}) {
  const s = p.state;
  const [filter, setFilter] = useState<Filter>('all');
  const [editing, setEditing] = useState<ImportItem>();
  const [bulk, setBulk] = useState<'block' | 'credential'>();
  const filters: Filter[] = [
    'all',
    'ready',
    'pending',
    'duplicate',
    'conflict',
    'ignored',
  ];
  const open = s.items.filter(
    (item) => !['ready', 'ignored'].includes(item.status),
  ).length;
  const ready = s.items.filter((item) => item.status === 'ready').length;
  const visible = s.items.filter(
    (item) => filter === 'all' || item.status === filter,
  );
  return (
    <>
      <ImportSteps step={2} />
      <div className="bar">
        <span className="muted">
          {s.filename} · {s.items.length} linhas
        </span>
        <span className="grow" />
        <Button disabled={s.busy} onClick={p.replace}>
          Trocar arquivo
        </Button>
      </div>
      <div className="chips">
        {filters.map((f) => (
          <Button
            key={f}
            className={filter === f ? 'on' : ''}
            onClick={() => setFilter(f)}
          >
            {f === 'all' ? 'Todas' : labels[f]} (
            {f === 'all'
              ? s.items.length
              : s.items.filter((item) => item.status === f).length}
            )
          </Button>
        ))}
      </div>
      {s.selected.length > 0 && (
        <div className="inventory-selection" role="status">
          <b>{s.selected.length} selecionada(s)</b>
          <Button onClick={() => setBulk('block')}>Definir bloco</Button>
          <Button onClick={() => setBulk('credential')}>
            Definir credencial
          </Button>
          <Button
            onClick={() =>
              void s.review(
                s.items.map((item) =>
                  s.selected.includes(item.row.line)
                    ? { ...item.row, ignored: true }
                    : item.row,
                ),
              )
            }
          >
            Ignorar
          </Button>
        </div>
      )}
      <DataTable aria-label="Prévia da importação">
        <thead>
          <tr>
            <th className="import-check"></th>
            <th>Linha</th>
            <th>Nome</th>
            <th>IP</th>
            <th>Papel</th>
            <th>Bloco</th>
            <th>Credencial</th>
            <th>Situação</th>
            <th>Motivo</th>
            <th>Ação</th>
          </tr>
        </thead>
        <tbody>
          {visible.map((item) => (
            <ImportRowView
              key={item.row.line}
              item={item}
              state={s}
              correct={() => setEditing(item)}
            />
          ))}
          {!visible.length && (
            <tr>
              <td colSpan={10}>Nenhuma linha nesta situação</td>
            </tr>
          )}
        </tbody>
      </DataTable>
      <div className="bar import-footer">
        <span className="muted">
          {open
            ? `Resolva ${open} linha${open === 1 ? '' : 's'} para continuar.`
            : 'Tudo pronto. Cadastrar não consulta os equipamentos.'}
        </span>
        <span className="grow" />
        <Button onClick={p.cancel}>Cancelar</Button>
        <Button
          kind="primary"
          disabled={s.busy || !!open || !ready}
          onClick={() => void s.confirm()}
        >
          Cadastrar {ready} equipamentos
        </Button>
      </div>
      {s.error && (
        <Button onClick={() => void s.review(s.items.map((item) => item.row))}>
          Atualizar prévia
        </Button>
      )}
      {editing && (
        <Correction
          item={editing}
          state={s}
          credentials={p.credentials}
          close={() => setEditing(undefined)}
        />
      )}
      {bulk && (
        <SelectionDialog
          mode={bulk}
          state={s}
          credentials={p.credentials}
          close={() => setBulk(undefined)}
        />
      )}
    </>
  );
}
