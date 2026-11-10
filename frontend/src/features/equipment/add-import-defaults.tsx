import type { useImportState } from './add-import-state';
import type { Credential } from './equipment-api';
import { Field } from '../../shared/ui/Field';
import { Select } from '../../shared/ui/Select';
type ImportFileProps = {
  state: ReturnType<typeof useImportState>;
  credentials: Credential[];
};
export function ImportDefaults(p: ImportFileProps) {
  const s = p.state;
  return (
    <section className="import-card">
      <h2>Aplicar às linhas sem valor</h2>
      <Field
        layout="form"
        label="Bloco"
        placeholder="Região, torre ou outro nome"
        value={s.block}
        onChange={(e) => s.setBlock(e.target.value)}
      />
      <label className="field">
        Credencial
        <Select
          aria-label="Credencial"
          value={s.credential}
          onChange={(e) => s.setCredential(e.target.value)}
        >
          <option value="">Sem credencial</option>
          {p.credentials.map((c) => (
            <option key={c.id} value={c.alias}>
              {c.alias}
            </option>
          ))}
        </Select>
      </label>
    </section>
  );
}
