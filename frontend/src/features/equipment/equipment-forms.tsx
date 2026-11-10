import { useEffect, useRef, useState } from 'react';
import { Button } from '../../shared/ui/Button';
import type {
  Credential,
  Device,
  DeviceInput,
  EquipmentResult,
} from './equipment-api';
import { useDraftProtection, useFieldErrors } from './equipment-draft';
import { ExistingCredentialSelect } from './equipment-form-credentials';
import { id } from './equipment-reading-format';
import { DeviceRoleSelect } from './equipment-role-select';
import {
  fieldForEquipmentCode,
  fieldMessages,
  validEquipmentAddress,
  validEquipmentText,
} from './equipment-validation';
import { RadioTransports } from './radio-contract';
export { ProtectedDialog } from '../access/protected-dialog';

type EditProps = {
  defaults: Record<string, string>;
  blocks?: string[];
  device: Device;
  credentials: Credential[];
  disabled: boolean;
  save: (d: DeviceInput) => Promise<EquipmentResult | undefined>;
};

function initialDraft(device: Device): DeviceInput {
  return {
    id: device.id,
    operation_id: id(),
    revision: device.revision,
    name: device.name,
    block: device.block || '',
    role: device.role || 'other',
    address: device.address,
    port: device.port,
    transport: RadioTransports.ssh,
    credential_id: device.credential_id || '',
    user: '',
    secret: '',
  };
}
function validate(draft: DeviceInput) {
  const errors: Record<string, string> = {};
  if (!validEquipmentText(draft.name)) errors.name = fieldMessages.name;
  if (!validEquipmentAddress(draft.address))
    errors.address = fieldMessages.address;
  if (!Number.isInteger(draft.port) || draft.port < 1 || draft.port > 65535)
    errors.port = fieldMessages.port;
  return errors;
}
function dirty(d: DeviceInput, device: Device) {
  return (
    d.name !== device.name ||
    d.block !== (device.block || '') ||
    d.role !== (device.role || 'other') ||
    d.address !== device.address ||
    d.port !== device.port ||
    d.credential_id !== (device.credential_id || '')
  );
}

export function DeviceForm(props: EditProps) {
  const { device, disabled } = props;
  const [draft, setDraft] = useState(() => initialDraft(device));
  const submitting = useRef(false);
  const alive = useRef(true);
  const { ref, setErrors, attrs, message } = useFieldErrors(disabled);
  const cancel = useDraftProtection(() => ({
    dirty: dirty(draft, device),
    blocked: submitting.current,
  }));
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const change = (values: Partial<DeviceInput>) =>
    setDraft({ ...draft, ...values });
  const submit = async () => {
    if (submitting.current || disabled) return;
    const errors = validate(draft);
    setErrors(errors);
    if (Object.keys(errors).length) return;
    submitting.current = true;
    try {
      const result = await props.save({
        ...draft,
        name: draft.name.trim(),
        address: draft.address.trim(),
      });
      if (alive.current && result && fieldForEquipmentCode[result.code]) {
        const field = fieldForEquipmentCode[result.code];
        setErrors({ [field]: fieldMessages[field] });
      }
    } finally {
      submitting.current = false;
    }
  };
  return (
    <form
      noValidate
      ref={ref}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
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
        <DeviceRoleSelect
          draft={draft}
          change={change}
          device={device}
          defaults={props.defaults}
        />
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
        <label className="field">
          Bloco
          <input
            aria-label="Bloco"
            value={draft.block}
            maxLength={80}
            list="device-block-suggestions"
            onChange={(e) => change({ block: e.target.value })}
          />
          <datalist id="device-block-suggestions">
            {(props.blocks || []).map((block) => (
              <option key={block} value={block} />
            ))}
          </datalist>
        </label>
        <ExistingCredentialSelect
          draft={draft}
          change={change}
          device={device}
          credentials={props.credentials}
          attrs={attrs}
          message={message}
        />
      </fieldset>
      <details className="equipment-help">
        <summary>Sobre o acesso</summary>
        <p>
          A senha salva permanece intacta. Para substituir um acesso
          compartilhado, use Credenciais. Alterar IP ou porta exige conferir a
          identidade novamente.
        </p>
      </details>
      <div className="dialog-actions">
        <Button disabled={disabled} onClick={cancel}>
          Cancelar
        </Button>
        <Button type="submit" className="btn primary" disabled={disabled}>
          Salvar equipamento
        </Button>
      </div>
    </form>
  );
}
