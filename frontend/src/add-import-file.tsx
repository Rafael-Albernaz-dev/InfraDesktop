import type { Credential } from './equipment-api';
import { inventoryImportAPI } from './presentation';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';
import { Field } from './ui/Field';
import { Select } from './ui/Select';
import type { useImportState } from './add-import-state';

export function ImportSteps({ step }: { step: number }) {
  return (
    <div className="import-steps">
      {['1. Arquivo', '2. Prévia', '3. Concluído'].map((label, i) => (
        <span key={label}>
          {i > 0 && <span aria-hidden="true">› </span>}
          {step === i + 1 ? <b>{label}</b> : label}
        </span>
      ))}
    </div>
  );
}
export function ImportFile(p: {
  state: ReturnType<typeof useImportState>;
  credentials: Credential[];
}) {
  const s = p.state;
  return (
    <>
      <ImportSteps step={1} />
      <div className="import-grid">
        <section className="import-card">
          <h2>Arquivo</h2>
          <p>
            Envie um arquivo <b>.xlsx</b>, <b>.csv</b> ou <b>.json</b>, com até
            500 equipamentos. Nada é gravado antes da sua confirmação.
          </p>
          <div className="actions">
            <Button
              kind="primary"
              disabled={s.busy}
              onClick={() => void s.choose()}
            >
              Escolher arquivo
            </Button>
            {(['xlsx', 'csv', 'json'] as const).map((format) => (
              <Button
                key={format}
                onClick={() => {
                  void inventoryImportAPI.template(format).then((code) => {
                    if (code !== 'OK' && code !== 'EXPORT_CANCELLED')
                      s.setError('Não foi possível salvar o modelo.');
                  });
                }}
              >
                Modelo .{format}
              </Button>
            ))}
          </div>
          <p className="muted">
            Senha nunca vai no arquivo: campo de senha faz o arquivo ser
            recusado. A credencial é escolhida entre as que já estão no cofre.
          </p>
        </section>
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
      </div>
      <div className="sp10" />
      <section className="import-card">
        <h2>Campos do modelo</h2>
        <p className="muted">No JSON, uma lista de objetos com estes campos.</p>
        <DataTable>
          <thead>
            <tr>
              <th>Campo</th>
              <th>Obrigatório</th>
              <th>Exemplo</th>
            </tr>
          </thead>
          <tbody>
            {[
              ['Nome', 'Sim', 'AP-LESTE-1'],
              ['IP', 'Sim', '192.0.2.31'],
              ['Porta', 'Não, padrão 22', '22'],
              ['Papel', 'Sim', 'AP'],
              ['Bloco', 'Não', 'Torre Leste'],
              ['Credencial', 'Não', 'leitura-radio'],
            ].map(([name, required, example]) => (
              <tr key={name}>
                <td>{name}</td>
                <td>{required}</td>
                <td>{example}</td>
              </tr>
            ))}
          </tbody>
        </DataTable>
      </section>
    </>
  );
}
