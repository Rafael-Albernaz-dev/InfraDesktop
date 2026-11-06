import { useState } from 'react';
import { AddEquipmentFields } from './add-equipment-fields';
import { ImportSteps } from './add-import-file';
import type { useImportState } from './add-import-state';
import { ImportPreviewTable } from './add-import-table';
import type { Credential } from './equipment-api';
import { roleGroups } from './equipment-role-options';
import type { ImportItem } from './presentation';
import { Button } from './ui/Button';
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
type CorrectionProps = {
  item: ImportItem;
  state: State;
  credentials: Credential[];
  close: () => void;
};
function Correction(p: CorrectionProps) {
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
type SelectionDialogProps = {
  mode: 'block' | 'credential';
  state: State;
  credentials: Credential[];
  close: () => void;
};
function SelectionDialog(p: SelectionDialogProps) {
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
type ImportPreviewProps = {
  state: State;
  credentials: Credential[];
  cancel: () => void;
  replace: () => void;
};
export function ImportPreview(p: ImportPreviewProps) {
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
      <PreviewHeader
        {...p}
        filter={filter}
        setFilter={setFilter}
        filters={filters}
      />
      <PreviewSelection s={s} setBulk={setBulk} />
      <ImportPreviewTable s={s} visible={visible} setEditing={setEditing} />
      <PreviewFooter {...p} open={open} ready={ready} />
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

type PreviewHeaderProps = ImportPreviewProps & {
  filter: Filter;
  setFilter: (f: Filter) => void;
  filters: Filter[];
};
function PreviewHeader(p: PreviewHeaderProps) {
  const s = p.state,
    { filter, setFilter, filters } = p;
  return (
    <>
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
    </>
  );
}
function PreviewSelection({
  s,
  setBulk,
}: {
  s: State;
  setBulk: (m: 'block' | 'credential') => void;
}) {
  return (
    <>
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
    </>
  );
}
function PreviewFooter(
  p: ImportPreviewProps & { open: number; ready: number },
) {
  const s = p.state,
    { open, ready } = p;
  return (
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
  );
}
