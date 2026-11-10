import { displayPort } from './equipment-port-format';
import { readingAge } from './equipment-reading-format';
import { exactNumber, formatDate } from '../radio/network-format';
import type { PPReadRow } from '../../platform/wails/presentation';
import { Button } from '../../shared/ui/Button';
import { DataBlock } from '../../shared/ui/DataBlock';
import { DataTable } from '../../shared/ui/DataTable';
export function EquipmentPortHistory({
  read,
  limited,
  at,
  now,
  cursor,
  setCursor,
  close,
}: {
  read?: { rows: PPReadRow[]; next: string; error: boolean };
  limited?: boolean;
  at?: string;
  now: number;
  cursor: string;
  setCursor: (c: string) => void;
  close: () => void;
}) {
  const historyChanges =
    read?.rows.filter((r) => r.kind === 'port_change') || [];
  return (
    <section aria-label="Histórico de portas salvo">
      <div className="sp10" />
      <DataBlock
        title="Histórico de portas"
        actions={
          <Button
            aria-label="Histórico de portas"
            aria-expanded
            onClick={() => close()}
          >
            Recolher
          </Button>
        }
        source="local"
        age={readingAge(at, now)}
        data={{
          state: 'ready',
          content: (
            <>
              {!read ? (
                <p role="status">Lendo histórico salvo…</p>
              ) : read.error ? (
                <p role="alert">Histórico indisponível.</p>
              ) : !historyChanges.length ? (
                <p>
                  {limited
                    ? 'Sem eventos de porta entre os registros retidos; histórico parcial.'
                    : 'Nenhuma mudança de porta detectada entre as leituras disponíveis.'}
                </p>
              ) : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Quando</th>
                      <th>Porta</th>
                      <th>De → Para</th>
                    </tr>
                  </thead>
                  <tbody>
                    {historyChanges.map((r, i) => (
                      <tr key={i}>
                        <td title={r.at}>{formatDate(r.at)}</td>
                        <td>{r.name}</td>
                        <td>
                          {displayPort(r.before_port)} (
                          {exactNumber(r.before_port?.link_downs)} quedas) →{' '}
                          {displayPort(r.port)} (
                          {exactNumber(r.port?.link_downs)} quedas)
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
              <div className="pager">
                <Button disabled={!cursor} onClick={() => setCursor('')}>
                  Início
                </Button>
                <span>
                  Página {cursor ? 2 : 1}
                  {!read?.next ? ' de 1' : ''}
                </span>
                <Button
                  aria-label="Próxima página do histórico de portas"
                  disabled={!read?.next}
                  onClick={() => setCursor(read?.next || '')}
                >
                  Próxima
                </Button>
              </div>
            </>
          ),
        }}
      />
    </section>
  );
}
