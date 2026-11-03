import { Button } from './desktop';
import { equipmentAPI as api } from './equipment-api';
import { EquipmentDetailPane } from './equipment-detail';
import {
  CredentialFormDialog,
  DeviceFormDialog,
  ResetTrustDialog,
  TrustConfirmDialog,
  VAULT_NOTICE,
} from './equipment-dialogs';
import { EquipmentInventoryPane } from './equipment-inventory';
import { useEquipmentState } from './equipment-state';
import { PPPoE } from './pppoe';
import { RadioRoles } from './radio-contract';

function EquipmentBanners(p: {
  s: ReturnType<typeof useEquipmentState>;
  locked: boolean;
  trustOpen: boolean;
}) {
  const { s, locked, trustOpen } = p;
  const plain =
    !s.shownForm && !s.credentialForm && !trustOpen && !s.resetTrust;
  return (
    <>
      {s.credentialError && (
        <p role="alert">
          {s.credentialError}{' '}
          <button onClick={() => s.reloadCredentials((n) => n + 1)}>
            Recarregar credenciais
          </button>
        </p>
      )}
      {s.selected && s.job?.code === 'REMOTE_AUTH_FAILED' && !s.shownForm && (
        <Button disabled={locked} onClick={() => s.setForm(s.selected)}>
          Escolher outra credencial
        </Button>
      )}
      {s.error && plain && (
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

function EquipmentDialogs(p: {
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

function EquipmentCapabilityBody(p: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  connected: boolean;
  primaryInHeader: boolean;
  locked: boolean;
  trustOpen: boolean;
}) {
  const { s, profile, activation, connected, primaryInHeader, locked } = p;
  const layoutClass = s.detailOpen
    ? 'equipment-detail-layout'
    : !s.selected
      ? 'inventory-without-detail'
      : '';
  return (
    <div className="equipment-capability">
      <EquipmentBanners s={s} locked={locked} trustOpen={p.trustOpen} />
      <div className={'pane-layout ' + layoutClass}>
        <EquipmentInventoryPane
          {...s}
          connected={connected}
          primaryInHeader={primaryInHeader}
          locked={locked}
        />
        {s.selected && (
          <EquipmentDetailPane
            {...s}
            selected={s.selected}
            connected={connected}
            locked={locked}
            profile={profile}
            activation={activation}
          />
        )}
      </div>
      {s.detailOpen && !s.selected && (
        <p role="status">Carregando equipamento…</p>
      )}
      {s.job?.state === 'RUNNING' && (
        <div className="inventory-notice" role="status">
          {s.job.kind === 'trust.inspect'
            ? 'Conferindo chave SSH sem enviar senha…'
            : 'Consulta SSH em curso…'}{' '}
          <Button
            onClick={() =>
              void s.perform(
                () => api.cancel(profile, activation, s.job!.id),
                (r) => s.setJob(r.data?.job),
                false,
              )
            }
          >
            Cancelar consulta
          </Button>
        </div>
      )}
      <EquipmentDialogs {...p} />
    </div>
  );
}

export function Equipment(props: {
  profile: string;
  activation: string;
  connected: boolean;
  createRequest: number;
  primaryInHeader?: boolean;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
}) {
  const { profile, activation, connected, primaryInHeader = false } = props;
  const s = useEquipmentState(props);
  if (!profile)
    return (
      <div className="empty-state">
        <h2>Selecione uma empresa</h2>
        <p>O inventário pertence à empresa ativa.</p>
      </div>
    );
  const locked = s.busy || !connected;
  const trustOpen = Boolean(
    !s.selected?.fingerprint &&
    s.selected?.role !== RadioRoles.ap &&
    s.selected?.role !== RadioRoles.cpe &&
    s.job?.kind === 'trust.inspect' &&
    s.job.state === 'SUCCEEDED' &&
    s.selected &&
    s.job.device_id === s.selected.id &&
    s.jobDevice?.revision === s.selected.revision,
  );
  if (s.ppOpen && s.selected)
    return (
      <PPPoE
        key={[profile, activation, s.selected.id].join('|')}
        profile={profile}
        activation={activation}
        device={s.selected}
        back={() => s.setPPOpen(false)}
        openEquipment={(id) => {
          s.setPPOpen(false);
          s.openById(id);
        }}
      />
    );
  return (
    <EquipmentCapabilityBody
      s={s}
      profile={profile}
      activation={activation}
      connected={connected}
      primaryInHeader={primaryInHeader}
      locked={locked}
      trustOpen={trustOpen}
    />
  );
}
