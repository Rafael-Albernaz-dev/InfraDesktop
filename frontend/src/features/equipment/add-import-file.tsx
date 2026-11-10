import { ImportDefaults } from './add-import-defaults';
import type { useImportState } from './add-import-state';
import { ImportTemplateFields } from './add-import-template';
import type { Credential } from './equipment-api';
import { inventoryImportAPI } from '../../platform/wails/presentation';
import { Button } from '../../shared/ui/Button';

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
type ImportFileProps = {
  state: ReturnType<typeof useImportState>;
  credentials: Credential[];
};
function ImportFilePicker(p: ImportFileProps) {
  const s = p.state;
  return (
    <section className="import-card">
      <h2>Arquivo</h2>
      <p>
        Envie um arquivo <b>.xlsx</b>, <b>.csv</b> ou <b>.json</b>, com até 500
        equipamentos. Nada é gravado antes da sua confirmação.
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
        Senha nunca vai no arquivo: campo de senha faz o arquivo ser recusado. A
        credencial é escolhida entre as que já estão no cofre.
      </p>
    </section>
  );
}
export function ImportFile(p: ImportFileProps) {
  return (
    <>
      <ImportSteps step={1} />
      <div className="import-grid">
        <ImportFilePicker {...p} />
        <ImportDefaults {...p} />
      </div>
      <div className="sp10" />
      <ImportTemplateFields />
    </>
  );
}
