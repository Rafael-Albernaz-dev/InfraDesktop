import { Button } from '../../shared/ui/Button';
import { Dialog } from '../../shared/ui/Dialog';
export function SharedCredentialConfirm(p: {
  links: number;
  disabled: boolean;
  cancel: () => void;
  confirm: () => void;
}) {
  return (
    <Dialog portal title="Substituir acesso?" close={p.cancel}>
      <p>
        Substituir o acesso usado por {p.links} equipamentos, inclusive
        inativos.
      </p>
      <div className="dialog-actions">
        <Button disabled={p.disabled} onClick={p.cancel}>
          Cancelar
        </Button>
        <Button kind="primary" disabled={p.disabled} onClick={p.confirm}>
          Substituir acesso
        </Button>
      </div>
    </Dialog>
  );
}
