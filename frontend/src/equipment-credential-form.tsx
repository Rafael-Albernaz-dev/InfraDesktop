import { CredentialFormFields } from './equipment-credential-fields';
import { Select } from './ui/Select';
import { DataBlock } from './ui/DataBlock';
import { Fragment, useEffect, useRef, useState } from 'react';
import { Button } from './desktop';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type {
  Credential,
  CredentialInput,
  EquipmentResult,
} from './equipment-api';
import { useDraftProtection, useFieldErrors } from './equipment-draft';
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
  defaultContextRef: React.MutableRefObject<string>;
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
    defaultContextRef,
    reloadCredentials,
    setCredentialError,
  } = props;
  return (
    <DataBlock
      title="Credencial padrão por tipo"
      source="cadastro local"
      data={{
        state: 'ready',
        content: (
          <dl className="kv">
            {DEFAULT_CREDENTIAL_ROLES.map((role) => (
              <Fragment key={role}>
                <dt>{role.toUpperCase()}</dt>
                <dd>
                  <Select
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
                          if (defaultContextRef.current !== context) return;
                          if (r.code === 'OK') reloadCredentials((n) => n + 1);
                          else setCredentialError(equipmentMessage(r.code));
                        })
                        .catch(() => {
                          if (defaultContextRef.current === context)
                            setCredentialError(
                              equipmentMessage('SERVICE_UNAVAILABLE'),
                            );
                        })
                        .finally(() => setDefaultBusy(false));
                    }}
                  >
                    <option value="">Selecione no cofre</option>
                    {credentials.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.alias}
                      </option>
                    ))}
                  </Select>
                </dd>
              </Fragment>
            ))}
          </dl>
        ),
      }}
    />
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
  existing: boolean;
}) {
  const { draft, setDraft, setSent, setErrors, submitting, alive, save, existing } = args;
  const errors: Record<string, string> = {};
  if (!validEquipmentText(draft.alias)) errors.alias = fieldMessages.alias;
  if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
  if ((!existing || draft.secret) && !validEquipmentSecret(draft.secret)) errors.secret = fieldMessages.secret;
  setErrors(errors);
  if (Object.keys(errors).length) return;
  submitting.current = true;
  const request = {
    ...draft,
    alias: draft.alias.trim(),
    user: draft.user.trim(),
  };
  setDraft({ ...draft, secret: '' });
  setSent(Boolean(draft.secret));
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
  const [sent, setSent] = useState(false);
  return { draft, setDraft, sent, setSent };
}
export function CredentialForm({
  credential,
  disabled,
  save,
  onDelete,
  usedReason,
}: {
  credential?: Credential;
  disabled: boolean;
  save: (d: CredentialInput) => Promise<EquipmentResult | undefined>;
  onDelete?: () => void;
  usedReason?: string;
}) {
  const { draft, setDraft, sent, setSent } =
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
      draft.secret !== '',
    blocked: submitting.current,
  }));
  return (
    <form
      noValidate
      ref={formRef}
      onSubmit={(e) => {
        e.preventDefault();
        if (submitting.current || disabled)
          return;
        submitCredentialForm({
          draft,
          setDraft,
          setSent,
          setErrors,
          submitting,
          alive,
          save,
          existing: Boolean(credential),
        });
      }}
    >
      <CredentialFormFields
        draft={draft}
        setDraft={setDraft}
        credential={credential}
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
        {credential && <>
          <Button disabled={disabled || Boolean(usedReason)} title={usedReason} onClick={onDelete}>Excluir credencial</Button>
          <span className="grow" />
        </>}
        <Button disabled={disabled} onClick={cancel}>
          Cancelar
        </Button>
        <Button
          type="submit"
          className="btn primary"
          disabled={disabled}
        >
          Salvar credencial
        </Button>
      </div>
    </form>
  );
}

export { CredentialPicker } from './equipment-credential-fields';
