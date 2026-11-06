import type { AddEquipmentFieldsProps } from './add-equipment-fields';
import { RadioRoles } from './radio-contract';
import { Select } from './ui/Select';
export function AddCredentialField(p: AddEquipmentFieldsProps) {
  const { row, change, credentials } = p;
  return (
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
  );
}
