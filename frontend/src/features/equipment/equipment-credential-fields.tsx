import { Select } from '../../shared/ui/Select';

import { useContext } from 'react';

import type { Credential, CredentialInput } from './equipment-api';
import { DraftProtection, useFieldErrors } from '../access/equipment-draft';

export function CredentialPicker({
  disabled,
  credentials,
  selected,
  select,
}: {
  disabled: boolean;
  credentials: Credential[];
  selected: Credential | 'new';
  select: (value: Credential | 'new') => void;
}) {
  const protection = useContext(DraftProtection);
  return (
    <Select
      disabled={disabled}
      aria-label="Credencial para editar"
      value={selected === 'new' ? '' : selected.id}
      onChange={(e) => {
        const target =
          credentials.find((c) => c.id === e.target.value) || 'new';
        protection?.attempt(() => select(target));
      }}
    >
      <option value="">Nova credencial</option>
      {credentials.map((c) => (
        <option key={c.id} value={c.id}>
          {c.alias} · {c.links} vínculos
        </option>
      ))}
    </Select>
  );
}
export function CredentialFormFields(props: {
  draft: CredentialInput;
  setDraft: (d: CredentialInput) => void;
  credential?: Credential;
  disabled: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const { draft, setDraft, credential, disabled, attrs, message } = props;
  return (
    <>
      <label className="field">
        Nome
        <input
          required
          maxLength={80}
          disabled={disabled}
          aria-label="Alias"
          {...attrs('alias')}
          value={draft.alias}
          onChange={(e) => setDraft({ ...draft, alias: e.target.value })}
        />
        {message('alias')}
      </label>
      <label className="field">
        Usuário
        <input
          required
          maxLength={80}
          disabled={disabled}
          aria-label="Usuário remoto"
          {...attrs('user')}
          value={draft.user}
          autoComplete="off"
          onChange={(e) => setDraft({ ...draft, user: e.target.value })}
        />
        {message('user')}
      </label>
      <label className="field">
        Senha
        <input
          type="password"
          required={!credential}
          maxLength={1024}
          autoComplete="new-password"
          disabled={disabled}
          aria-label="Senha remota"
          {...attrs('secret')}
          value={draft.secret}
          onChange={(e) => setDraft({ ...draft, secret: e.target.value })}
        />
        {message('secret')}
      </label>
      {credential && (
        <p className="muted">Deixe vazio para manter a senha atual.</p>
      )}
    </>
  );
}
