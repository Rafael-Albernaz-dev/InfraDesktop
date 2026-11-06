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
import { AddEquipment } from './add-equipment';
import { useState } from 'react';
import { SessionRoute } from './session-navigation';
import type { SessionRouteTarget } from './session-navigation';

function EquipmentCapabilityBody(p: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  connected: boolean;
  primaryInHeader: boolean;
  credentialsPage?: boolean;
  addPage?: boolean;
  onAdd?: () => void;
  onAddBack?: () => void;
  locked: boolean;
  trustOpen: boolean;
  onBackReady?: (back?: () => void) => void;
}) {
  const { s, profile, activation, connected, primaryInHeader, locked } = p;
  const adding = p.addPage;
  return (
    <div className="equipment-capability">
      <EquipmentBanners s={s} locked={locked} trustOpen={p.trustOpen} />
      <div>
        {adding ? (
          <AddEquipment key={[profile, activation].join("|")} s={s} profile={profile} activation={activation}
            locked={locked} back={() => p.onAddBack?.()} />
        ) : p.credentialsPage ? (
          <EquipmentCredentials
            s={s}
            profile={profile}
            activation={activation}
            locked={locked}
          />
        ) : (
          <>
            <EquipmentInventoryPane
              key={[profile, activation].join('|')}
              {...s}
              profile={profile}
              activation={activation}
              connected={connected}
              primaryInHeader={primaryInHeader}
              locked={locked}
              onAdd={p.onAdd}
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
  addPage?: boolean;
  onAdd?: () => void;
  onAddBack?: () => void;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
  onBackReady?: (back?: () => void) => void;
  onBlockedChange?: (blocked: boolean) => void;
};

export function Equipment(props: EquipmentProps) {
  const external = useContext(SessionRoute);
  const [localSession, setLocalSession] = useState<SessionRouteTarget>();
  return (
    <SessionRoute.Provider value={external || {
      active: !!localSession,
      target: localSession,
      open: setLocalSession,
      back: () => setLocalSession(undefined),
      leave: () => setLocalSession(undefined),
    }}>
    <EquipmentContext
      key={props.credentialsPage ? 'credentials' : 'equipment'}
      {...props}
    />
    </SessionRoute.Provider>
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
  const [localAdd, setLocalAdd] = useState(false);
  const adding = !!props.addPage || localAdd;
  const s = useEquipmentState({ ...props, addVisible: adding });
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
      addPage={adding}
      onAdd={() => props.onAdd ? props.onAdd() : setLocalAdd(true)}
      onAddBack={() => { setLocalAdd(false); props.onAddBack?.(); }}
      locked={locked}
      trustOpen={trustOpen}
      onBackReady={props.onBackReady}
    />
  );
}
