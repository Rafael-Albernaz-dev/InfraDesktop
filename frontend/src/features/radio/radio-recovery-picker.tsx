import { Select } from '../../shared/ui/Select';
import type { useRecoveryState } from './radio-credential-state';
export function RadioRecoveryPicker({
  state,
  save,
}: {
  state: ReturnType<typeof useRecoveryState>;
  save: (id: string) => Promise<void>;
}) {
  const { busy, target, credentials } = state;
  return (
    <label>
      Credencial de consulta
      <Select
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
      </Select>
    </label>
  );
}
