import { Button } from './desktop';
import type { Credential, Device, DeviceInput } from './equipment-api';
import type { useFieldErrors } from './equipment-draft';
import { RadioRoles } from './radio-contract';

const RADIO_ROLE_VALUES: string[] = Object.values(RadioRoles);

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
        <select
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
        </select>
        {message('credential_id')}
      </label>
      {!device && <p className="muted">Somente credenciais desta empresa.</p>}
    </>
  );
}

export function InlineCredentialInputs(props: {
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  vault: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const { draft, change, vault, attrs, message } = props;
  return (
    <>
      <label className="field">
        Usuário
        <input
          required
          maxLength={80}
          aria-label="Usuário"
          {...attrs('user')}
          value={draft.user}
          autoComplete="off"
          onChange={(e) => change({ user: e.target.value })}
        />
        {message('user')}
      </label>
      <label className="field">
        Senha
        <input
          type="password"
          required
          maxLength={1024}
          aria-label="Senha"
          {...attrs('secret')}
          value={draft.secret}
          autoComplete="new-password"
          disabled={!vault}
          onChange={(e) => change({ secret: e.target.value })}
        />
        {message('secret')}
      </label>
    </>
  );
}

export function DeviceCredentialSection(props: {
  existing: boolean;
  setExisting: (v: boolean) => void;
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  device?: Device;
  credentials: Credential[];
  vault: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const { existing, setExisting, draft, change, device, vault } = props;
  return (
    <>
      {existing ? (
        <ExistingCredentialSelect {...props} />
      ) : (
        <InlineCredentialInputs {...props} />
      )}
      {!device && !RADIO_ROLE_VALUES.includes(draft.role || '') && (
        <Button
          kind="flat"
          disabled={!vault && existing}
          onClick={() => {
            setExisting(!existing);
            change({ user: '', secret: '', credential_id: '' });
          }}
        >
          {existing ? 'Informar usuário e senha' : 'Usar credencial existente'}
        </Button>
      )}
    </>
  );
}
