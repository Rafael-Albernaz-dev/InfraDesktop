import { Select } from './ui/Select';
import type { Credential, Device, DeviceInput } from './equipment-api';
import type { useFieldErrors } from './equipment-draft';
export function ExistingCredentialSelect(props: {
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  device?: Device;
  credentials: Credential[];
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const { draft, change, device, credentials, attrs, message } = props;
  return (
    <>
      <label className="field">
        Credencial de consulta
        <Select
          aria-label="Credencial de consulta"
          {...attrs('credential_id')}
          value={draft.credential_id}
          required={!device}
          onChange={(e) => change({ credential_id: e.target.value })}
        >
          <option value="">{device ? 'Não vinculada' : 'Selecione'}</option>
          {credentials.map((c) => (
            <option key={c.id} value={c.id}>
              {c.alias} · {c.user}
            </option>
          ))}
        </Select>
        {message('credential_id')}
      </label>
      {!device && <p className="muted">Somente credenciais desta empresa.</p>}
    </>
  );
}
