import { useId } from 'react';
import type { Credential } from './equipment-api';
import type { ImportRow } from './presentation';
import { Field } from './ui/Field';
import { Select } from './ui/Select';
import { RoleOptions } from './equipment-roles';
import { RadioRoles } from './radio-contract';
import type { useFieldErrors } from './equipment-draft';

export function AddEquipmentFields(p: {
  row: ImportRow;
  change: (v: Partial<ImportRow>) => void;
  credentials: Credential[];
  blocks?: string[];
  credentialById?: boolean;
  addCredential?: () => void;
  correction?: boolean;
  disabled?: boolean;
  validation?: Pick<ReturnType<typeof useFieldErrors>, 'attrs' | 'message'>;
}) {
  const { row, change, credentials } = p;
  const blockList = useId();
  return (
    <div
      className={
        p.correction ? 'add-form-grid correction-form' : 'add-form-grid'
      }
    >
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
      <label className="field">
        {p.correction ? 'Credencial' : 'Credencial de consulta'}
        <Select
          aria-label={p.correction ? 'Credencial' : 'Credencial de consulta'}
          disabled={p.disabled}
          value={row.credential}
          onChange={(e) => {
            if (e.target.value === '__add__') p.addCredential?.();
            else change({ credential: e.target.value });
          }}
        >
          <option value="">Sem credencial</option>
          {row.credential &&
            row.credential !== '__inline__' &&
            !credentials.some(
              (c) => (p.credentialById ? c.id : c.alias) === row.credential,
            ) && (
              <option value={row.credential}>
                {row.credential} (não encontrada)
              </option>
            )}
          {credentials.map((c) => (
            <option key={c.id} value={p.credentialById ? c.id : c.alias}>
              {c.alias}
            </option>
          ))}
          {p.addCredential && (
            <>
              <option value="__add__">Adicionar credencial</option>
              {!Object.values<string>(RadioRoles).includes(row.role) && (
                <option value="__inline__">Informar usuário e senha</option>
              )}
            </>
          )}
        </Select>
      </label>
    </div>
  );
}
