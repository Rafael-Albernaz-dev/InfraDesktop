import { EquipmentBanners, EquipmentDialogs } from './equipment-surfaces';
import { useContext, useEffect, useLayoutEffect } from 'react';
import { DetailTitle } from './ui/DetailTitle';
import { Button } from './desktop';
import { equipmentAPI as api } from './equipment-api';
import { EquipmentDetailPane } from './equipment-detail';
import { EquipmentInventoryPane } from './equipment-inventory';
import { useEquipmentState } from './equipment-state';
import { EquipmentCredentials } from './equipment-credentials';
import { RadioRoles } from './radio-contract';

function EquipmentCapabilityBody(p: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  connected: boolean;
  primaryInHeader: boolean;
  credentialsPage?: boolean;
  locked: boolean;
  trustOpen: boolean;
  onBackReady?: (back?: () => void) => void;
}) {
  const { s, profile, activation, connected, primaryInHeader, locked } = p;
  return (
    <div className="equipment-capability">
      <EquipmentBanners s={s} locked={locked} trustOpen={p.trustOpen} />
      <div>
        {p.credentialsPage ? (
          <EquipmentCredentials
            s={s}
            profile={profile}
            activation={activation}
            locked={locked}
          />
        ) : (
          <>
            <EquipmentInventoryPane
              {...s}
              connected={connected}
              primaryInHeader={primaryInHeader}
              locked={locked}
            />
            {s.selected && s.detailOpen && (
              <EquipmentDetailPane
                {...s}
                key={s.selected.id}
                selected={s.selected}
                connected={connected}
                locked={locked}
                profile={profile}
                activation={activation}
                onBackReady={p.onBackReady}
              />
            )}
          </>
        )}
      </div>
      {s.detailOpen && !s.selected && (
        <p role="status">Carregando equipamento…</p>
      )}
      {s.job?.state === 'RUNNING' && (
        <div className="note" role="status">
          {s.job.kind === 'trust.inspect'
            ? 'Conferindo chave SSH sem enviar senha…'
            : 'Leitura em curso…'}{' '}
          <Button
            onClick={() =>
              void s.perform(
                () => api.status(profile, activation, s.job!.id),
                (r) => s.setJob(r.data?.job),
                false,
              )
            }
          >
            Conferir andamento
          </Button>
        </div>
      )}
      <EquipmentDialogs {...p} />
    </div>
  );
}

export type EquipmentProps = {
  profile: string;
  activation: string;
  connected: boolean;
  createRequest: number;
  primaryInHeader?: boolean;
  credentialsPage?: boolean;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
  onBackReady?: (back?: () => void) => void;
  onBlockedChange?: (blocked: boolean) => void;
};

export function Equipment(props: EquipmentProps) {
  return (
    <EquipmentContext
      key={props.credentialsPage ? 'credentials' : 'equipment'}
      {...props}
    />
  );
}

function EquipmentContext(props: EquipmentProps) {
  const {
    profile,
    activation,
    connected,
    primaryInHeader = false,
    onBlockedChange,
  } = props;
  const s = useEquipmentState(props);
  useEffect(() => {
    onBlockedChange?.(s.busy || s.job?.state === 'RUNNING');
    return () => onBlockedChange?.(false);
  }, [s.busy, s.job?.state, onBlockedChange]);
  const setTitle = useContext(DetailTitle);
  useLayoutEffect(() => {
    setTitle(
      !props.credentialsPage && s.detailOpen ? s.selected?.name || '' : '',
    );
    return () => setTitle('');
  }, [setTitle, props.credentialsPage, s.detailOpen, s.selected?.name]);
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
  return (
    <EquipmentCapabilityBody
      s={s}
      profile={profile}
      activation={activation}
      connected={connected}
      primaryInHeader={primaryInHeader}
      credentialsPage={props.credentialsPage}
      locked={locked}
      trustOpen={trustOpen}
      onBackReady={props.onBackReady}
    />
  );
}
