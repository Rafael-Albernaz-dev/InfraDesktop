import { Button } from './ui/Button';
import type { Credential } from './equipment-api';

export function CredentialDeleteConfirm(p: {
  credential: Credential;
  locked: boolean;
  cancel: () => void;
  confirm: () => void;
}) {
  return <>
    <p>A credencial {p.credential.alias} será removida do cofre. Esta ação não pode ser desfeita.</p>
    <div className="dialog-actions">
      <Button disabled={p.locked} onClick={p.cancel}>Cancelar</Button>
      <Button kind="primary" disabled={p.locked} onClick={p.confirm}>Excluir credencial</Button>
    </div>
  </>;
}
