import type { useImportState } from './add-import-state';
import { roleGroups } from './equipment-role-options';
import type { ImportItem } from './presentation';
import { Button } from './ui/Button';
import { Checkbox } from './ui/Checkbox';
import { DataTable } from './ui/DataTable';
type State = ReturnType<typeof useImportState>;
const labels: Record<ImportItem['status'], string> = {
  ready: 'Pronta',
  pending: 'Pendente',
  duplicate: 'Duplicada',
  conflict: 'Conflito',
  ignored: 'Ignorada',
};
type ImportRowViewProps = {
  item: ImportItem;
  state: State;
  correct: () => void;
};

function ImportRowView(p: ImportRowViewProps) {
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
        {labels[item.status] || 'Situação desconhecida'}
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
export function ImportPreviewTable({
  s,
  visible,
  setEditing,
}: {
  s: State;
  visible: ImportItem[];
  setEditing: React.Dispatch<React.SetStateAction<ImportItem | undefined>>;
}) {
  return (
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
  );
}
