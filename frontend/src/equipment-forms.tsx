import { useEffect, useRef, useState } from 'react';
import { Button } from './desktop';
import { DeviceCredentialSection } from './equipment-form-credentials';
import type {
  Credential,
  Device,
  DeviceInput,
  EquipmentResult,
} from './equipment-api';
import {
  ProtectedDialog,
  useDraftProtection,
  useFieldErrors,
} from './equipment-draft';
import { pendingEnrollments } from './equipment-receipts';
import { id } from './equipment-reading';
import {
  fieldForEquipmentCode,
  fieldMessages,
  validEquipmentAddress,
  validEquipmentSecret,
  validEquipmentText,
} from './equipment-validation';
import { RadioRoles, RadioTransports } from './radio-contract';

export { ProtectedDialog };

const RADIO_ROLE_VALUES: string[] = [
  RadioRoles.ap,
  RadioRoles.cpe,
  RadioRoles.ptp,
];

function validateDeviceDraft(
  draft: DeviceInput,
  existing: boolean,
  device?: Device,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!validEquipmentText(draft.name)) errors.name = fieldMessages.name;
  if (!validEquipmentAddress(draft.address))
    errors.address = fieldMessages.address;
  if (!Number.isInteger(draft.port) || draft.port < 1 || draft.port > 65535)
    errors.port = fieldMessages.port;
  if (existing && !device && !draft.credential_id)
    errors.credential_id = fieldMessages.credential_id;
  if (!existing) {
    if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
    if (!validEquipmentSecret(draft.secret))
      errors.secret = fieldMessages.secret;
  }
  return errors;
}

function UncertainEnrollmentNotice(props: {
  disabled: boolean;
  checking: boolean;
  setChecking: (v: boolean) => void;
  reconcile: (
    device: string,
    operation: string,
  ) => Promise<EquipmentResult | undefined>;
  draft: DeviceInput;
  contextKey: string;
  alive: React.MutableRefObject<boolean>;
  setUncertain: (v: boolean) => void;
  setRetry: (v: boolean) => void;
}) {
  const {
    disabled,
    checking,
    setChecking,
    reconcile,
    draft,
    contextKey,
    alive,
    setUncertain,
    setRetry,
  } = props;
  return (
    <div role="status" className="inventory-notice">
      <p>Cadastro ainda não confirmado. Confira o resultado antes de fechar.</p>
      <Button
        disabled={disabled || checking}
        onClick={() => {
          setChecking(true);
          void reconcile(draft.id, draft.operation_id).then((r) => {
            if (r?.code === 'OK' && r.data?.operation_state === 'COMMITTED')
              pendingEnrollments.delete(contextKey);
            if (!alive.current) return;
            setChecking(false);
            if (r?.code === 'OK' && r.data?.operation_state === 'NOT_FOUND') {
              setUncertain(false);
              setRetry(true);
            }
          });
        }}
      >
        Conferir cadastro
      </Button>
    </div>
  );
}

function DeviceRoleSelect(props: {
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  device?: Device;
  defaults: Record<string, string>;
  setExisting: (v: boolean) => void;
}) {
  const { draft, change, device, defaults, setExisting } = props;
  return (
    <label className="field">
      Papel do equipamento
      <select
        aria-label="Papel do equipamento"
        value={draft.role}
        onChange={(e) => {
          const role = e.target.value;
          if (!device && RADIO_ROLE_VALUES.includes(role)) {
            setExisting(true);
            change({
              role,
              credential_id: defaults[role] || '',
              user: '',
              secret: '',
            });
          } else change({ role });
        }}
      >
        <option value="other">Outro</option>
        <option value="concentrator">Concentrador</option>
        <option value={RadioRoles.ap}>AP</option>
        <option value={RadioRoles.cpe}>CPE</option>
        <option value={RadioRoles.ptp}>PTP (somente cadastro)</option>
      </select>
    </label>
  );
}

function DeviceFormFields(props: {
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  device?: Device;
  defaults: Record<string, string>;
  existing: boolean;
  setExisting: (v: boolean) => void;
  credentials: Credential[];
  vault: boolean;
  disabled: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
}) {
  const { draft, change, disabled, attrs, message } = props;
  return (
    <fieldset className="device-fields" disabled={disabled}>
      <label className="field">
        Nome
        <input
          aria-label="Nome"
          {...attrs('name')}
          value={draft.name}
          maxLength={80}
          required
          onChange={(e) => change({ name: e.target.value })}
        />
        {message('name')}
      </label>
      <DeviceRoleSelect {...props} />
      <label className="field">
        IP de gerenciamento
        <input
          aria-label="IP de gerenciamento"
          {...attrs('address')}
          value={draft.address}
          required
          onChange={(e) => change({ address: e.target.value })}
          placeholder="IPv4 ou IPv6 literal"
        />
        {message('address')}
      </label>
      <details className="equipment-port">
        <summary>Porta SSH · {draft.port}</summary>
        <label className="field">
          Porta SSH
          <input
            type="number"
            aria-label="Porta SSH"
            min={1}
            max={65535}
            {...attrs('port')}
            value={draft.port}
            required
            onChange={(e) => change({ port: Number(e.target.value) })}
          />
          {message('port')}
        </label>
      </details>
      <DeviceCredentialSection {...props} />
    </fieldset>
  );
}

function isDraftDirty(
  draft: DeviceInput,
  device: Device | undefined,
  existing: boolean,
  uncertain: boolean,
): boolean {
  return (
    uncertain ||
    draft.name !== (device?.name || '') ||
    draft.role !== (device?.role || 'other') ||
    draft.address !== (device?.address || '') ||
    draft.port !== (device?.port || 22) ||
    draft.credential_id !== (device?.credential_id || '') ||
    draft.user !== '' ||
    draft.secret !== '' ||
    existing !== Boolean(device)
  );
}

function executeSaveRequest(
  request: DeviceInput,
  direct: boolean,
  contextKey: string,
  alive: React.MutableRefObject<boolean>,
  submitting: React.MutableRefObject<boolean>,
  setErrors: (e: Record<string, string>) => void,
  setUncertain: (v: boolean) => void,
  save: (d: DeviceInput) => Promise<EquipmentResult | undefined>,
) {
  void (async () => {
    try {
      const result = await save(request);
      request.secret = '';
      if (alive.current && result && fieldForEquipmentCode[result.code]) {
        const field = fieldForEquipmentCode[result.code];
        setErrors({ [field]: fieldMessages[field] });
      }
      if (direct) {
        const unknown =
          !result ||
          [
            'SERVICE_UNAVAILABLE',
            'DEADLINE_EXCEEDED',
            'REVISION_CONFLICT',
          ].includes(result.code);
        if (!unknown) pendingEnrollments.delete(contextKey);
        if (alive.current) setUncertain(unknown);
      }
    } catch {
      if (alive.current && direct) setUncertain(true);
    } finally {
      request.secret = '';
      submitting.current = false;
    }
  })();
}

function submitDeviceForm(args: {
  draft: DeviceInput;
  existing: boolean;
  device?: Device;
  contextKey: string;
  setDraft: (d: DeviceInput) => void;
  setSent: (v: boolean) => void;
  setErrors: (e: Record<string, string>) => void;
  setUncertain: (v: boolean) => void;
  submitting: React.MutableRefObject<boolean>;
  alive: React.MutableRefObject<boolean>;
  save: (d: DeviceInput) => Promise<EquipmentResult | undefined>;
}) {
  const {
    draft,
    existing,
    device,
    contextKey,
    setDraft,
    setSent,
    setErrors,
    setUncertain,
    submitting,
    alive,
    save,
  } = args;
  const errors = validateDeviceDraft(draft, existing, device);
  setErrors(errors);
  if (Object.keys(errors).length) return;
  submitting.current = true;
  const request = {
    ...draft,
    name: draft.name.trim(),
    address: draft.address.trim(),
    user: existing ? '' : draft.user.trim(),
    secret: existing ? '' : draft.secret,
    credential_id: existing ? draft.credential_id : '',
  };
  const direct = !device;
  if (direct)
    pendingEnrollments.set(contextKey, {
      id: draft.id,
      operation_id: draft.operation_id,
    });
  setDraft({ ...draft, secret: '' });
  setSent(!existing);
  executeSaveRequest(
    request,
    direct,
    contextKey,
    alive,
    submitting,
    setErrors,
    setUncertain,
    save,
  );
}

function DeviceFormFooter({
  device,
  sent,
  existing,
  disabled,
  vault,
  cancel,
}: {
  device?: Device;
  sent: boolean;
  existing: boolean;
  disabled: boolean;
  vault: boolean;
  cancel?: () => void;
}) {
  return (
    <>
      {device && (
        <details className="equipment-help">
          <summary>Sobre o acesso</summary>
          <p>
            A senha salva permanece intacta. Para substituir um acesso
            compartilhado, use Credenciais. Alterar IP ou porta exige conferir a
            identidade novamente.
          </p>
        </details>
      )}
      {sent && !existing && !disabled && (
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
          disabled={disabled || (!device && !existing && !vault)}
        >
          {device ? 'Salvar equipamento' : 'Salvar e conectar'}
        </button>
      </div>
    </>
  );
}

function useInitialDeviceDraft(device?: Device, contextKey = '') {
  const receipt = !device ? pendingEnrollments.get(contextKey) : undefined;
  const [draft, setDraft] = useState<DeviceInput>(() => ({
    id: device?.id || receipt?.id || id(),
    operation_id: receipt?.operation_id || id(),
    revision: device?.revision || 0,
    name: device?.name || '',
    role: device?.role || 'other',
    address: device?.address || '',
    port: device?.port || 22,
    transport: RadioTransports.ssh,
    credential_id: device?.credential_id || '',
    user: '',
    secret: '',
  }));
  const [uncertain, setUncertain] = useState(Boolean(!device && receipt));
  return { draft, setDraft, uncertain, setUncertain };
}

function DeviceFormBody(props: {
  uncertain: boolean;
  disabled: boolean;
  checking: boolean;
  setChecking: (v: boolean) => void;
  reconcile: (
    device: string,
    operation: string,
  ) => Promise<EquipmentResult | undefined>;
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  contextKey: string;
  alive: React.MutableRefObject<boolean>;
  setUncertain: (v: boolean) => void;
  retry: boolean;
  setRetry: (v: boolean) => void;
  device?: Device;
  defaults: Record<string, string>;
  existing: boolean;
  setExisting: (v: boolean) => void;
  credentials: Credential[];
  vault: boolean;
  attrs: ReturnType<typeof useFieldErrors>['attrs'];
  message: ReturnType<typeof useFieldErrors>['message'];
  sent: boolean;
  cancel?: () => void;
}) {
  if (props.uncertain) return <UncertainEnrollmentNotice {...props} />;
  return (
    <>
      {props.retry && (
        <p role="status">
          Ainda não há confirmação. Para tentar com o mesmo cadastro, preencha
          os dados novamente.
        </p>
      )}
      <DeviceFormFields {...props} />
      <DeviceFormFooter
        device={props.device}
        sent={props.sent}
        existing={props.existing}
        disabled={props.disabled}
        vault={props.vault}
        cancel={props.cancel}
      />
    </>
  );
}

export function DeviceForm(props: {
  defaults: Record<string, string>;
  device?: Device;
  credentials: Credential[];
  disabled: boolean;
  vault: boolean;
  contextKey: string;
  save: (d: DeviceInput) => Promise<EquipmentResult | undefined>;
  reconcile: (
    device: string,
    operation: string,
  ) => Promise<EquipmentResult | undefined>;
}) {
  const { device, disabled, contextKey, save } = props;
  const { draft, setDraft, uncertain, setUncertain } = useInitialDeviceDraft(
    device,
    contextKey,
  );
  const [existing, setExisting] = useState(Boolean(device));
  const [checking, setChecking] = useState(false);
  const [retry, setRetry] = useState(false);
  const submitting = useRef(false);
  const { ref: formRef, setErrors, attrs, message } = useFieldErrors(disabled);
  const [sent, setSent] = useState(false);
  const cancel = useDraftProtection(() => ({
    dirty: isDraftDirty(draft, device, existing, uncertain),
    blocked: submitting.current || checking,
  }));
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const change = (values: Partial<DeviceInput>) =>
    setDraft({ ...draft, ...values });
  return (
    <form
      noValidate
      ref={formRef}
      onSubmit={(e) => {
        e.preventDefault();
        if (submitting.current || disabled || uncertain) return;
        submitDeviceForm({
          draft,
          existing,
          device,
          contextKey,
          setDraft,
          setSent,
          setErrors,
          setUncertain,
          submitting,
          alive,
          save,
        });
      }}
    >
      <DeviceFormBody
        {...props}
        uncertain={uncertain}
        checking={checking}
        setChecking={setChecking}
        draft={draft}
        change={change}
        alive={alive}
        setUncertain={setUncertain}
        retry={retry}
        setRetry={setRetry}
        existing={existing}
        setExisting={setExisting}
        attrs={attrs}
        message={message}
        sent={sent}
        cancel={cancel}
      />
    </form>
  );
}
