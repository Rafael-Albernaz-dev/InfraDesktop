import { Fragment } from 'react';
import { Button } from '../../shared/ui/Button';
import type {
  Credential,
  CredentialInput,
  EquipmentResult,
} from './equipment-api';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import { CredentialFormFields } from './equipment-credential-fields';
import { id } from './equipment-reading-format';
import { RadioRoles } from './radio-contract';
import { SharedCredentialConfirm } from './shared-credential-confirm';
import { DataBlock } from '../../shared/ui/DataBlock';
import { Select } from '../../shared/ui/Select';
import { useCredentialDraft } from './use-credential-draft';
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

type CredentialFormProps = {
  credential?: Credential;
  disabled: boolean;
  save: (draft: CredentialInput) => Promise<EquipmentResult | undefined>;
  onDelete?: () => void;
  usedReason?: string;
};
export function CredentialForm({
  credential,
  disabled,
  save,
  onDelete,
  usedReason,
}: CredentialFormProps) {
  const form = useCredentialDraft({ credential, disabled, save }),
    { draft, setDraft, sent, validation } = form;
  const { ref: formRef, attrs, message } = validation;
  return (
    <>
      <form
        noValidate
        ref={formRef}
        onSubmit={(event) => {
          event.preventDefault();
          form.submit(false);
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
          {credential && (
            <>
              <Button
                disabled={disabled || Boolean(usedReason)}
                title={usedReason}
                onClick={onDelete}
              >
                Excluir credencial
              </Button>
              <span className="grow" />
            </>
          )}
          <Button disabled={disabled} onClick={form.cancel}>
            Cancelar
          </Button>
          <Button type="submit" className="btn primary" disabled={disabled}>
            Salvar credencial
          </Button>
        </div>
      </form>
      {form.confirming && (
        <SharedCredentialConfirm
          links={credential?.links || 0}
          disabled={disabled}
          cancel={() => form.setConfirming(false)}
          confirm={() => form.submit(true)}
        />
      )}
    </>
  );
}
export { CredentialPicker } from './equipment-credential-fields';
