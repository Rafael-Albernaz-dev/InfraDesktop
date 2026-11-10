import { useId } from 'react';
import { AddCredentialField } from './add-equipment-access-field';
import type { Credential } from './equipment-api';
import type { useFieldErrors } from './equipment-draft';
import { RoleOptions } from './equipment-roles';
import type { ImportRow } from '../../platform/wails/presentation';
import { Field } from '../../shared/ui/Field';
import { Select } from '../../shared/ui/Select';

export type AddEquipmentFieldsProps = {
  row: ImportRow;
  change: (v: Partial<ImportRow>) => void;
  credentials: Credential[];
  blocks?: string[];
  credentialById?: boolean;
  addCredential?: () => void;
  correction?: boolean;
  disabled?: boolean;
  validation?: Pick<ReturnType<typeof useFieldErrors>, 'attrs' | 'message'>;
};
function AddIdentityFields(p: AddEquipmentFieldsProps) {
  const { row, change } = p;
  return (
    <>
      <Field
        layout="form"
        label="Nome"
        disabled={p.disabled}
        aria-label="Nome"
        {...p.validation?.attrs('name')}
        value={row.name}
        maxLength={80}
        onChange={(e) => change({ name: e.target.value })}
      >
        {p.validation?.message('name')}
      </Field>
      <Field
        layout="form"
        label="IP de gerenciamento"
        disabled={p.disabled}
        aria-label="IP de gerenciamento"
        {...p.validation?.attrs('address')}
        value={row.address}
        onChange={(e) => change({ address: e.target.value })}
      >
        {p.validation?.message('address')}
      </Field>
      <Field
        layout="form"
        label="Porta SSH"
        disabled={p.disabled}
        aria-label="Porta SSH"
        {...p.validation?.attrs('port')}
        type="number"
        min={1}
        max={65535}
        value={row.port}
        onChange={(e) => change({ port: e.target.value })}
      >
        {p.validation?.message('port')}
      </Field>
    </>
  );
}
function AddRoleFields(p: AddEquipmentFieldsProps) {
  const { row, change } = p;
  const blockList = useId();
  return (
    <>
      <label className="field">
        Papel do equipamento
        <Select
          aria-label="Papel do equipamento"
          disabled={p.disabled}
          value={row.role}
          onChange={(e) => change({ role: e.target.value })}
        >
          <option value="">Selecione</option>
          <RoleOptions />
        </Select>
      </label>
      <Field
        layout="form"
        label="Bloco"
        disabled={p.disabled}
        value={row.block}
        maxLength={80}
        placeholder="Região, torre ou outro nome"
        list={blockList}
        onChange={(e) => change({ block: e.target.value })}
      >
        <datalist id={blockList}>
          {(p.blocks || []).map((block) => (
            <option key={block} value={block} />
          ))}
        </datalist>
      </Field>
    </>
  );
}
export function AddEquipmentFields(p: AddEquipmentFieldsProps) {
  return (
    <div
      className={
        p.correction ? 'add-form-grid correction-form' : 'add-form-grid'
      }
    >
      <AddIdentityFields {...p} />
      <AddRoleFields {...p} />
      <AddCredentialField {...p} />
    </div>
  );
}
