import { useContext, useEffect, useRef, useState } from 'react';
import { Button } from './desktop';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type {
  Credential,
  CredentialInput,
  EquipmentResult,
} from './equipment-api';
import {
  DraftProtection,
  useDraftProtection,
  useFieldErrors,
} from './equipment-draft';
import { id } from './equipment-reading';
import {
  fieldForEquipmentCode,
  fieldMessages,
  validEquipmentSecret,
  validEquipmentText,
} from './equipment-validation';
import { RadioRoles } from './radio-contract';

const DEFAULT_CREDENTIAL_ROLES = [
  RadioRoles.ap,
  RadioRoles.cpe,
  RadioRoles.ptp,
] as const;

export function DefaultCredentialDetails(props: {
  profile: string;
  activation: string;
  locked: boolean;
  defaultBusy: boolean;
  setDefaultBusy: (v: boolean) => void;
  credentialDefaults: Record<string, string>;
  credentials: Credential[];
  defaultContext: React.MutableRefObject<string>;
  reloadCredentials: React.Dispatch<React.SetStateAction<number>>;
  setCredentialError: (v: string) => void;
}) {
  const {
    profile,
    activation,
    locked,
    defaultBusy,
    setDefaultBusy,
    credentialDefaults,
    credentials,
    defaultContext,
    reloadCredentials,
    setCredentialError,
  } = props;
  return (
    <details className="equipment-help">
      <summary>Credencial padrão por classe</summary>
      {DEFAULT_CREDENTIAL_ROLES.map((role) => (
        <label className="field" key={role}>
          {role.toUpperCase()}
          <select
            aria-label={'Credencial padrão ' + role.toUpperCase()}
            disabled={locked || defaultBusy}
            value={credentialDefaults[role] || ''}
            onChange={(e) => {
              const selected = e.target.value,
                context = [profile, activation].join('|');
              if (!selected) return;
              setDefaultBusy(true);
              void api
                .setDefault(profile, activation, role, selected, id())
                .then((r) => {
                  if (defaultContext.current !== context) return;
                  if (r.code === 'OK') reloadCredentials((n) => n + 1);
                  else setCredentialError(equipmentMessage(r.code));
                })
                .catch(() => {
                  if (defaultContext.current === context)
                    setCredentialError(equipmentMessage('SERVICE_UNAVAILABLE'));
                })
                .finally(() => setDefaultBusy(false));
            }}
          >
            <option value="">Selecione no cofre</option>
            {credentials.map((c) => (
              <option key={c.id} value={c.id}>
                {c.alias} · {c.user}
              </option>
            ))}
          </select>
        </label>
      ))}
    </details>
  );
}

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
    <select
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
    </select>
  );
}

function submitCredentialForm(args: {
  draft: CredentialInput;
  setDraft: (d: CredentialInput) => void;
  setSent: (v: boolean) => void;
  setErrors: (e: Record<string, string>) => void;
  submitting: React.MutableRefObject<boolean>;
  alive: React.MutableRefObject<boolean>;
  save: (d: CredentialInput) => Promise<EquipmentResult | undefined>;
}) {
  const { draft, setDraft, setSent, setErrors, submitting, alive, save } = args;
  const errors: Record<string, string> = {};
  if (!validEquipmentText(draft.alias)) errors.alias = fieldMessages.alias;
  if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
  if (!validEquipmentSecret(draft.secret)) errors.secret = fieldMessages.secret;
  setErrors(errors);
  if (Object.keys(errors).length) return;
  submitting.current = true;
  const request = {
    ...draft,
    alias: draft.alias.trim(),
    user: draft.user.trim(),
  };
  setDraft({ ...draft, secret: '' });
  setSent(true);
  void (async () => {
    try {
      const result = await save(request);
      if (alive.current && result && fieldForEquipmentCode[result.code]) {
        const field = fieldForEquipmentCode[result.code];
        setErrors({ [field]: fieldMessages[field] });
      }
    } catch {
      // perform reports transport failures; never retain the submitted secret.
    } finally {
      request.secret = '';
      submitting.current = false;
    }
  })();
}

function CredentialFormFields(props: {
  draft: CredentialInput;
  setDraft: (d: CredentialInput) => void;
  credential?: Credential;
  confirmed: boolean;
  setConfirmed: (v: boolean) => void;
  disabled: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const {
    draft,
    setDraft,
    credential,
    confirmed,
    setConfirmed,
    disabled,
    attrs,
    message,
  } = props;
  return (
    <>
      <label className="field">
        Alias
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
        Usuário remoto
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
        Senha remota
        <input
          type="password"
          required
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
        <label className="check">
          <input
            disabled={disabled}
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
          />
          Substituir o acesso usado por {credential.links} equipamentos,
          inclusive inativos.
        </label>
      )}
    </>
  );
}

function useInitialCredentialDraft(credential?: Credential) {
  const [draft, setDraft] = useState<CredentialInput>(() => ({
    id: credential?.id || id(),
    operation_id: id(),
    revision: credential?.revision || 0,
    alias: credential?.alias || '',
    user: credential?.user || '',
    secret: '',
    affected_links: credential?.links || 0,
  }));
  const [confirmed, setConfirmed] = useState(false);
  const [sent, setSent] = useState(false);
  return { draft, setDraft, confirmed, setConfirmed, sent, setSent };
}

export function CredentialForm({
  credential,
  disabled,
  save,
}: {
  credential?: Credential;
  disabled: boolean;
  save: (d: CredentialInput) => Promise<EquipmentResult | undefined>;
}) {
  const { draft, setDraft, confirmed, setConfirmed, sent, setSent } =
    useInitialCredentialDraft(credential);
  const submitting = useRef(false);
  const { ref: formRef, setErrors, attrs, message } = useFieldErrors(disabled);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const cancel = useDraftProtection(() => ({
    dirty:
      draft.alias !== (credential?.alias || '') ||
      draft.user !== (credential?.user || '') ||
      draft.secret !== '' ||
      confirmed,
    blocked: submitting.current,
  }));
  return (
    <form
      noValidate
      ref={formRef}
      onSubmit={(e) => {
        e.preventDefault();
        if (submitting.current || disabled || (credential && !confirmed))
          return;
        submitCredentialForm({
          draft,
          setDraft,
          setSent,
          setErrors,
          submitting,
          alive,
          save,
        });
      }}
    >
      <CredentialFormFields
        draft={draft}
        setDraft={setDraft}
        credential={credential}
        confirmed={confirmed}
        setConfirmed={setConfirmed}
        disabled={disabled}
        attrs={attrs}
        message={message}
      />
      {sent && !disabled && (
        <p role="status">
          Digite a senha novamente para tentar salvar. A falha ao salvar não
          verifica a senha no equipamento.
        </p>
      )}
      <div className="dialog-actions">
        <Button disabled={disabled} onClick={cancel}>
          Cancelar
        </Button>
        <button
          className="btn primary"
          disabled={disabled || Boolean(credential && !confirmed)}
        >
          Salvar credencial
        </button>
      </div>
    </form>
  );
}
