import { Button } from '../../shared/ui/Button';
import { Dialog } from '../../shared/ui/Dialog';
import type { Credential, Device, EquipmentResult, Job } from './equipment-api';
import { equipmentAPI as api } from './equipment-api';
import { VAULT_NOTICE } from '../access/vault-notice';
import { DeviceForm, ProtectedDialog } from './equipment-forms';
import { id } from './equipment-reading-format';
export type EquipmentPerformFn = (
  call: () => Promise<EquipmentResult>,
  after?: (r: EquipmentResult) => void,
  refreshList?: boolean,
  inlineFields?: boolean,
) => Promise<EquipmentResult | undefined>;

function DialogNotices({
  error,
  vault,
  ready,
}: {
  error: string;
  vault: boolean;
  ready: boolean;
}) {
  return (
    <>
      {error && (
        <p className="inline-error" role="alert">
          {error}
        </p>
      )}
      {!vault && ready && !error && (
        <p role="alert" className="inline-error">
          {VAULT_NOTICE}
        </p>
      )}
    </>
  );
}

type TrustConfirmProps = {
  selected: Device;
  job: Job;
  error: string;
  locked: boolean;
  profile: string;
  activation: string;
  confirmOperation: React.MutableRefObject<{ job: string; operation: string }>;
  dismissTrust: () => void;
  perform: EquipmentPerformFn;
  setJob: (j: Job | undefined) => void;
  setSelected: (d: Device | undefined) => void;
  setConnectRequest: (d: Device | undefined) => void;
};

function runConfirmTrust(p: TrustConfirmProps) {
  if (p.confirmOperation.current.job !== p.job.id)
    p.confirmOperation.current = { job: p.job.id, operation: id() };
  void p.perform(
    () =>
      api.confirm(
        p.profile,
        p.activation,
        p.selected.id,
        p.confirmOperation.current.operation,
        p.job.id,
        p.job.fingerprint || '',
        p.selected.revision,
      ),
    (r) => {
      p.setJob(undefined);
      if (r.data?.device) {
        p.setSelected(r.data.device);
        p.setConnectRequest(r.data.device);
      }
    },
    false,
  );
}

export function TrustConfirmDialog(p: TrustConfirmProps) {
  return (
    <Dialog portal title="Primeira conexão" close={p.dismissTrust}>
      {p.error && (
        <p className="inline-error" role="alert">
          {p.error}
        </p>
      )}
      <p>
        Confiar em {p.selected.name} ({p.selected.address}:{p.selected.port}) e
        conectar?
      </p>
      <p>
        Esta é a primeira conexão. Mudanças posteriores na chave bloquearão a
        conexão.
      </p>
      <details className="equipment-help">
        <summary>Detalhes avançados</summary>
        <p>A primeira chave ainda não foi verificada por outro canal.</p>
        <p className="fingerprint mono">{p.job.fingerprint}</p>
      </details>
      <div className="dialog-actions">
        <Button disabled={p.locked} onClick={p.dismissTrust}>
          Cancelar
        </Button>
        <Button
          kind="primary"
          disabled={p.locked}
          onClick={() => runConfirmTrust(p)}
        >
          Confiar neste equipamento e conectar
        </Button>
      </div>
    </Dialog>
  );
}

export function ResetTrustDialog(p: {
  selected: Device;
  error: string;
  busy: boolean;
  locked: boolean;
  profile: string;
  activation: string;
  resetOperation: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
  setSelected: (d: Device | undefined) => void;
  setJob: (j: Job | undefined) => void;
  perform: EquipmentPerformFn;
}) {
  const reset = () =>
    void p.perform(
      () =>
        api.resetTrust(
          p.profile,
          p.activation,
          p.selected.id,
          p.resetOperation.current,
          p.selected.revision,
        ),
      (r) => {
        if (r.data?.device) p.setSelected(r.data.device);
        p.setResetTrust(false);
        p.setJob(undefined);
      },
      false,
    );
  return (
    <Dialog
      portal
      title="Redefinir confiança?"
      close={() => {
        if (!p.busy) p.setResetTrust(false);
      }}
    >
      <p>
        Use somente após confirmar uma troca legítima de chave no destino{' '}
        {p.selected.address}:{p.selected.port}. A próxima conexão exigirá um
        novo aceite; isto não verifica a identidade por outro canal.
      </p>
      {p.error && (
        <p role="alert" className="inline-error">
          {p.error}
        </p>
      )}
      <div className="dialog-actions">
        <Button disabled={p.locked} onClick={() => p.setResetTrust(false)}>
          Cancelar
        </Button>
        <Button kind="primary" disabled={p.locked} onClick={reset}>
          Remover chave salva
        </Button>
      </div>
    </Dialog>
  );
}

export function DeviceFormDialog(p: {
  shownForm: Device;
  closeForm: () => void;
  error: string;
  vault: boolean;
  ready: boolean;
  credentials: Credential[];
  credentialDefaults: Record<string, string>;
  blocks: string[];
  locked: boolean;
  profile: string;
  activation: string;
  perform: EquipmentPerformFn;
  reloadCredentials: React.Dispatch<React.SetStateAction<number>>;
  showSavedDevice: (r: EquipmentResult) => void;
}) {
  return (
    <ProtectedDialog title="Editar equipamento" close={p.closeForm}>
      <DialogNotices error={p.error} vault={p.vault} ready={p.ready} />
      <DeviceForm
        key={p.shownForm.id}
        device={p.shownForm}
        credentials={p.credentials}
        defaults={p.credentialDefaults}
        blocks={p.blocks}
        disabled={p.locked}
        save={(d) =>
          p.perform(
            async () => {
              const r = await api.save(p.profile, p.activation, d);
              if (r.code === 'OK') p.reloadCredentials((n) => n + 1);
              return r;
            },
            p.showSavedDevice,
            true,
            true,
          )
        }
      />
    </ProtectedDialog>
  );
}

export { CredentialFormDialog } from '../inventory/credential-form-dialog';
