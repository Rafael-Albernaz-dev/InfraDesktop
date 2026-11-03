import {
  createRecoveryChoice,
  createRecoverySave,
  useRecoveryAutoload,
  useRecoveryState,
} from './radio-credential-state';
import type { RadioRecoveryProps } from './radio-credential-state';

export function RadioCredentialRecovery(props: RadioRecoveryProps) {
  const state = useRecoveryState(props);
  const choose = createRecoveryChoice(state),
    save = createRecoverySave(state);
  useRecoveryAutoload(state, choose);
  const { message, open, busy, target, credentials } = state;
  return (
    <div>
      {message && <p role="status">{message}</p>}
      {open ? (
        <label>
          Credencial de consulta
          <select
            aria-label="Outra credencial do equipamento"
            disabled={busy}
            value={target?.credential_id || ''}
            onChange={(event) => void save(event.target.value)}
          >
            <option value="">Selecione</option>
            {credentials.map((credential) => (
              <option key={credential.id} value={credential.id}>
                {credential.alias} · {credential.user}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <button disabled={busy} onClick={() => void choose()}>
          Escolher outra
        </button>
      )}
    </div>
  );
}
