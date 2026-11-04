import { queryFailureOwnsDetail } from './equipment-reading-status';
import { RadioCodes } from './radio-contract';
import { Button } from './ui/Button';
import {
  CredentialFormDialog,
  DeviceFormDialog,
  ResetTrustDialog,
  TrustConfirmDialog,
  VAULT_NOTICE,
} from './equipment-dialogs';
import type { useEquipmentState } from './equipment-state';
export function EquipmentBanners(p: {
  s: ReturnType<typeof useEquipmentState>;
  locked: boolean;
  trustOpen: boolean;
}) {
  const { s, locked, trustOpen } = p;
  const operationError = queryFailureOwnsDetail(s) ? '' : s.error;
  const plain =
    !s.shownForm && !s.credentialForm && !trustOpen && !s.resetTrust;
  return (
    <>
      {s.credentialError && (
        <p role="alert">
          {s.credentialError}{' '}
          <Button
            type="submit"
            onClick={() => s.reloadCredentials((n) => n + 1)}
          >
            Recarregar credenciais
          </Button>
        </p>
      )}
      {s.selected &&
        s.job?.code === RadioCodes.REMOTE_AUTH_FAILED &&
        !s.shownForm && (
          <Button disabled={locked} onClick={() => s.setForm(s.selected)}>
            Escolher outra credencial
          </Button>
        )}
      {operationError && plain && (
        <p className="inline-error operation-error" role="alert">
          {s.error}
        </p>
      )}
      {s.listError && plain && (
        <div className="inventory-notice">
          <p className="inline-error" role="alert">
            {s.listError}
          </p>
          <Button disabled={locked} onClick={() => s.reload((n) => n + 1)}>
            Tentar novamente
          </Button>
        </div>
      )}
      {!s.vault && s.ready && !s.error && !s.listError && plain && (
        <p className="inventory-notice">{VAULT_NOTICE}</p>
      )}
    </>
  );
}
export function EquipmentDialogs(p: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  locked: boolean;
  trustOpen: boolean;
}) {
  const { s, profile, activation, locked, trustOpen } = p;
  return (
    <>
      {trustOpen && s.selected && s.job && (
        <TrustConfirmDialog
          {...s}
          selected={s.selected}
          job={s.job}
          locked={locked}
          profile={profile}
          activation={activation}
        />
      )}
      {s.resetTrust && s.selected && (
        <ResetTrustDialog
          {...s}
          selected={s.selected}
          locked={locked}
          profile={profile}
          activation={activation}
        />
      )}
      {s.shownForm && (
        <DeviceFormDialog
          {...s}
          shownForm={s.shownForm}
          locked={locked}
          profile={profile}
          activation={activation}
        />
      )}
      {s.credentialForm && (
        <CredentialFormDialog
          {...s}
          credentialForm={s.credentialForm}
          locked={locked}
          profile={profile}
          activation={activation}
        />
      )}
    </>
  );
}
