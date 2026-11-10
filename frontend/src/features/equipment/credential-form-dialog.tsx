import { useRef, useState } from 'react';
import { CredentialDeleteConfirm } from './credential-delete-confirm';
import type { Credential } from './equipment-api';
import { equipmentAPI as api } from './equipment-api';
import {
  CredentialForm,
  CredentialPicker,
  DefaultCredentialDetails,
} from './equipment-credential-form';
import type { EquipmentPerformFn } from './equipment-dialogs';
import { ProtectedDialog } from './equipment-forms';
import { id } from './equipment-reading-format';

function DialogNotices(p: { error: string; vault: boolean; ready: boolean }) {
  return (
    <>
      {p.error && (
        <p className="inline-error" role="alert">
          {p.error}
        </p>
      )}
      {!p.vault && p.ready && !p.error && (
        <p role="alert" className="inline-error">
          Cofre indisponível. Confira a credencial do serviço nesta instalação.
        </p>
      )}
    </>
  );
}

export function CredentialFormDialog(p: {
  credentialForm: Credential | 'new';
  setCredentialForm: (v: Credential | 'new' | undefined) => void;
  error: string;
  vault: boolean;
  ready: boolean;
  profile: string;
  activation: string;
  locked: boolean;
  defaultBusy: boolean;
  setDefaultBusy: (v: boolean) => void;
  credentialDefaults: Record<string, string>;
  credentials: Credential[];
  defaultContextRef: React.MutableRefObject<string>;
  reloadCredentials: React.Dispatch<React.SetStateAction<number>>;
  setCredentialError: (v: string) => void;
  perform: EquipmentPerformFn;
}) {
  const [deleting, setDeleting] = useState(false);
  const operation = useRef(id());
  const credential = p.credentialForm === 'new' ? undefined : p.credentialForm;
  const isDefault =
    credential && Object.values(p.credentialDefaults).includes(credential.id);
  const usedReason = credential?.links
    ? `Em uso por ${credential.links} equipamento(s)`
    : isDefault
      ? 'Credencial padrão por tipo'
      : undefined;
  return (
    <ProtectedDialog
      showClose={!credential}
      title={
        deleting
          ? 'Excluir credencial?'
          : credential
            ? 'Editar credencial'
            : 'Credenciais de consulta'
      }
      close={() => p.setCredentialForm(undefined)}
    >
      <DialogNotices error={p.error} vault={p.vault} ready={p.ready} />
      <div hidden={deleting}>
        {!credential && (
          <>
            <p>
              Reutilizáveis somente nesta empresa. Substituir aqui não altera a
              senha no equipamento.
            </p>
            <DefaultCredentialDetails {...p} />
            <CredentialPicker
              disabled={p.locked}
              credentials={p.credentials}
              selected={p.credentialForm}
              select={p.setCredentialForm}
            />
          </>
        )}
        <CredentialForm
          key={p.credentialForm === 'new' ? 'new' : p.credentialForm.id}
          credential={credential}
          disabled={p.locked || !p.vault}
          usedReason={usedReason}
          onDelete={() => setDeleting(true)}
          save={(d) =>
            p.perform(
              () => api.putCredential(p.profile, p.activation, d),
              () => {
                p.setCredentialForm(undefined);
                p.reloadCredentials((n) => n + 1);
              },
              true,
              true,
            )
          }
        />
      </div>
      {deleting && credential && (
        <CredentialDeleteConfirm
          credential={credential}
          locked={p.locked}
          cancel={() => setDeleting(false)}
          confirm={() =>
            void p.perform(
              () =>
                api.deleteCredential(
                  p.profile,
                  p.activation,
                  credential.id,
                  operation.current,
                  credential.revision,
                ),
              () => {
                p.setCredentialForm(undefined);
                p.reloadCredentials((n) => n + 1);
              },
              true,
            )
          }
        />
      )}
    </ProtectedDialog>
  );
}
