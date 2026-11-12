import { DataBlock } from '../../shared/ui/DataBlock';
import { DataTable } from '../../shared/ui/DataTable';
import { equipmentMessage } from '../equipment/equipment-api';
export { RadioIndexCredentialErrors } from './radio-link-credential-errors';
import { Button } from '../../shared/ui/Button';
import type { RadioLinkState } from './radio-link-state';
import { RadioModes } from '../equipment/radio-contract';

export function RadioIndexRefresh({ state }: { state: RadioLinkState }) {
  const { blocked, radio, editing, saving, now, link, index } = state;
  return (
    <>
      {!editing && (!link || index) && (
        <div className="radio-actions">
          <p>
            {index
              ? `Índice de ${Math.max(0, Math.floor((now - Date.parse(index.observed_at)) / 60000))} min atrás`
              : 'Índice ainda não lido'}
          </p>
          <Button
            type="submit"
            disabled={blocked || radio.busy || saving}
            onClick={() => void radio.start(RadioModes.index)}
          >
            Atualizar índice
          </Button>
        </div>
      )}
    </>
  );
}

export function RadioIndexPagination({ state }: { state: RadioLinkState }) {
  const { radio, indexPage, index } = state;
  return (
    <>
      {!radio.busy && index && (index.total || 0) > 8 && (
        <div className="radio-actions">
          <span>
            APs {(index.offset || 0) + 1}–
            {Math.min((index.offset || 0) + 8, index.total || 0)} de{' '}
            {index.total}
          </span>
          <Button
            type="submit"
            disabled={!index.offset}
            onClick={() => void indexPage(Math.max(0, (index.offset || 0) - 8))}
          >
            APs anteriores
          </Button>
          <Button
            type="submit"
            disabled={!index.more}
            onClick={() => void indexPage(index.next_offset || 0)}
          >
            Próximos APs
          </Button>
        </div>
      )}
    </>
  );
}
export function RadioLinkMatchReason({ state }: { state: RadioLinkState }) {
  const { radio, editing, link } = state;
  return (
    <>
      {!link && !editing && !radio.busy && radio.view?.match_reason && (
        <p
          role="status"
          className={
            radio.view.proposal && !state.index?.complete
              ? 'diverge'
              : undefined
          }
        >
          {radio.view.match_reason}
        </p>
      )}
    </>
  );
}

export function RadioIndexTable({ state }: { state: RadioLinkState }) {
  const { index, now, radio, editing } = state;
  if (!index || editing || radio.busy) return null;
  const age = Math.max(
    0,
    Math.floor((now - Date.parse(index.observed_at)) / 60000),
  );
  return (
    <DataBlock
      title="Índice de APs"
      source="local"
      age={`há ${age} min`}
      data={{
        state: 'ready',
        content: (
          <DataTable>
            <thead>
              <tr>
                <th>AP</th>
                <th>Clientes</th>
                <th>Leitura</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(index.targets).map(([id, code]) => (
                <tr key={id}>
                  <td>{index.labels?.[id] || id}</td>
                  <td>{code === 'OK' ? 'Não informado nesta leitura' : '—'}</td>
                  <td className={code === 'OK' ? undefined : 'diverge'}>
                    {code === 'OK' ? `há ${age} min` : equipmentMessage(code)}
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ),
      }}
    />
  );
}
