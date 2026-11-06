import type { Device } from './equipment-api';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import { SessionHeader } from './pppoe-session-header';
import { SessionAuthInfo, SessionConnectivityTab } from './pppoe-session-info';
import type { PPPoESessionDetailProps } from './pppoe-session-props';
import type { PPObservation, PPRow } from './presentation';
import { RadioLinkPanel } from './radio';
import { SessionChain } from './session-chain';
import { DataBlock } from './ui/DataBlock';
import { Tabs } from './ui/Tabs';

type SessionInformationTabProps = {
  selected: PPRow;
  current: PPObservation;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  openEquipment?: (id: string) => void;
};
function SessionInformationTab(p: SessionInformationTabProps) {
  const { selected, current, profile, activation, device, disabled } = p;
  const isOffline = selected.state === 'OFFLINE';
  return (
    <>
      <div className="session-grid">
        <SessionAuthInfo
          selected={selected}
          current={current}
          profile={profile}
          activation={activation}
          device={device}
          openEquipment={p.openEquipment}
          fastTrack={p.fastTrack}
        />
        <SessionConnectivityTab
          profile={profile}
          activation={activation}
          device={device}
          observation={current.observation_id}
          row={selected.row_id}
          uptime={selected.uptime}
          disabled={disabled}
          isOffline={isOffline}
          openEquipment={p.openEquipment}
        />
      </div>
      <div className="session-grid">
        {(['log', 'secret'] as const).map((evidence) => (
          <PPPoEReadPanel
            key={evidence}
            profile={profile}
            activation={activation}
            device={device}
            observation={current.observation_id}
            rows={[]}
            logRow={selected.row_id}
            evidence={evidence}
            panel
            blocked={disabled || isOffline}
          />
        ))}
      </div>
    </>
  );
}
function SessionDetailNav({
  detailTab,
  setDetailTab,
}: {
  detailTab: 'traffic' | 'information' | 'link' | 'history';
  setDetailTab: (t: 'traffic' | 'information' | 'link' | 'history') => void;
}) {
  return (
    <Tabs
      label="Detalhes da sessão"
      items={[
        ['traffic', 'Tráfego'],
        ['information', 'Informações'],
        ['link', 'Enlace'],
        ['history', 'Histórico'],
      ]}
      value={detailTab}
      onChange={setDetailTab}
    />
  );
}
function SessionTrafficTab(p: PPPoESessionDetailProps) {
  const { selected, current, profile, activation, device, disabled } = p;
  if (selected.state === 'OFFLINE')
    return (
      <DataBlock
        title="Tráfego"
        source={`SSH (${device.name})`}
        data={{
          state: 'ready',
          content: <p>Sem leitura: sessão OFFLINE</p>,
        }}
      />
    );
  return (
    <>
      <PPPoEReadPanel
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        rows={[selected.row_id]}
        traffic
        panel
        blocked={disabled}
      />
      <div className="session-grid">
        <PPPoEReadPanel
          profile={profile}
          activation={activation}
          device={device}
          observation={current.observation_id}
          rows={[selected.row_id]}
          panel
          bandwidthOnly
          blocked={disabled}
        />
        <PPPoEReadPanel
          profile={profile}
          activation={activation}
          device={device}
          observation={current.observation_id}
          rows={[]}
          panel
          allowTop
          blocked={disabled}
        />
      </div>
    </>
  );
}
function SessionDetailContent(p: PPPoESessionDetailProps) {
  const {
    selected,
    current,
    profile,
    activation,
    device,
    disabled,
    fastTrack,
    openEquipment,
  } = p;
  if (p.detailTab === 'history')
    return (
      <DataBlock
        title="Histórico de conexões"
        source="coleta local"
        data={{ state: 'unbuilt', slice: 'L-PULSO', label: 'Não feito' }}
      />
    );
  if (p.detailTab === 'traffic') return <SessionTrafficTab {...p} />;
  if (p.detailTab === 'link')
    return (
      <RadioLinkPanel
        session
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        row={selected.row_id}
        pppoeUptime={formatPPUptime(selected.uptime)}
        blocked={disabled || selected.state === 'OFFLINE'}
        onOpenEquipment={openEquipment}
      />
    );
  return (
    <SessionInformationTab
      selected={selected}
      current={current}
      profile={profile}
      activation={activation}
      device={device}
      disabled={disabled}
      fastTrack={fastTrack}
      openEquipment={openEquipment}
    />
  );
}
export function PPPoESessionDetail(p: PPPoESessionDetailProps) {
  const {
    current,
    profile,
    activation,
    device,
    selected,
    openEquipment,
    detailTab,
    setDetailTab,
  } = p;
  return (
    <section className="session-screen" aria-label="Detalhes da sessão">
      <SessionHeader {...p} />
      <SessionChain
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        row={selected.row_id}
        open={openEquipment}
      />
      <SessionDetailNav detailTab={detailTab} setDetailTab={setDetailTab} />
      <SessionDetailContent {...p} />
    </section>
  );
}
