import type { AddEquipmentProps } from './add-equipment';
import { ImportFile, ImportSteps } from './add-import-file';
import { ImportPreview } from './add-import-preview';
import { SingleEquipmentForm } from './add-single-equipment-form';
import { Button } from '../../shared/ui/Button';
import type { AddEquipmentModel } from './use-add-equipment-page';
type AddBodyProps = AddEquipmentProps & { model: AddEquipmentModel };
function UncertainEnrollment(p: AddBodyProps) {
  const { enrollment } = p.model;
  return (
    <div role="status" className="inventory-notice">
      <p>Cadastro ainda não confirmado. Confira o resultado antes de fechar.</p>
      <Button
        disabled={enrollment.checking || p.locked}
        onClick={() => void enrollment.reconcile()}
      >
        Conferir cadastro
      </Button>
    </div>
  );
}
function ImportReceipt(p: AddBodyProps) {
  const { s } = p.model;
  if (!s.receipt) return null;
  return (
    <>
      <ImportSteps step={3} />
      <section className="import-card">
        <h2>Importação concluída</h2>
        <p>
          <b>{s.receipt.created} equipamentos cadastrados</b>,{' '}
          {s.receipt.ignored} linhas ignoradas. Nenhuma consulta foi feita.
        </p>
        <div className="actions">
          <Button kind="primary" onClick={p.back}>
            Ver equipamentos
          </Button>
          <Button onClick={s.reset}>Importar outro arquivo</Button>
        </div>
      </section>
    </>
  );
}
function ImportFlow(p: AddBodyProps) {
  const { s, back, guard } = p.model;
  if (s.receipt) return <ImportReceipt {...p} />;
  if (s.items.length)
    return (
      <ImportPreview
        state={s}
        credentials={p.s.credentials}
        cancel={back}
        replace={() => guard.attempt(s.reset)}
      />
    );
  return <ImportFile state={s} credentials={p.s.credentials} />;
}
export function AddEquipmentBody(p: AddBodyProps) {
  const { tab, enrollment } = p.model;
  if (tab !== 'one') return <ImportFlow {...p} />;
  if (enrollment.uncertain) return <UncertainEnrollment {...p} />;
  return <SingleEquipmentForm {...p} />;
}
