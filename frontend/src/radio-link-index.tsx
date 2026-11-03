import type { RadioLinkState } from './radio-link-state';
import { RadioCodes, RadioModes } from './radio-contract';
import { RadioCredentialRecovery } from './radio-credential-recovery';
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
          <button
            disabled={blocked || radio.busy || saving}
            onClick={() => void radio.start(RadioModes.index)}
          >
            Atualizar índice
          </button>
        </div>
      )}
    </>
  );
}
export function RadioIndexCredentialErrors({
  state,
}: {
  state: RadioLinkState;
}) {
  const { profile, activation, radio, changed, index } = state;
  return (
    <>
      {!radio.busy &&
        index &&
        Object.entries(index.targets)
          .filter(([, code]) => code === RadioCodes.REMOTE_AUTH_FAILED)
          .map(([target]) => (
            <div key={target}>
              <p>AP {index.labels?.[target] || 'cadastrado'}</p>
              <RadioCredentialRecovery
                profile={profile}
                activation={activation}
                deviceID={target}
                changed={changed}
              />
            </div>
          ))}
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
          <button
            disabled={!index.offset}
            onClick={() => void indexPage(Math.max(0, (index.offset || 0) - 8))}
          >
            APs anteriores
          </button>
          <button
            disabled={!index.more}
            onClick={() => void indexPage(index.next_offset || 0)}
          >
            Próximos APs
          </button>
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
        <p role="status">{radio.view.match_reason}</p>
      )}
    </>
  );
}
