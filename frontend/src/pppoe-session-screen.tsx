import { DataBlock } from './ui/DataBlock';
import { SessionAuthInfo, SessionConnectivityTab } from './pppoe-session-info';
import { Button } from './ui/Button';
import { Tabs } from './ui/Tabs';
import type { Device } from './equipment-api';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import type { PPObservation, PPRow } from './presentation';
import { RadioLinkPanel } from './radio';
import { SessionChain } from './session-chain';

function SessionInformationTab(p: {
  selected: PPRow;
  current: PPObservation;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  openEquipment?: (id: string) => void;
}) {
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
export function PPPoESessionDetail(p: {
  selected: PPRow;
  current: PPObservation;
  detailHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  detailTab: 'traffic' | 'information' | 'link' | 'history';
  setDetailTab: (t: 'traffic' | 'information' | 'link' | 'history') => void;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  detailIntentRef: React.MutableRefObject<number>;
  select: (r: PPRow | undefined) => void;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  openEquipment?: (id: string) => void;
  back: () => void;
}) {
  const {
    selected,
    current,
    detailHeadingRef,
    detailTab,
    setDetailTab,
    profile,
    activation,
    device,
    disabled,
    fastTrack,
    detailIntentRef,
    select,
    openEquipment,
  } = p;
  return (
    <section className="session-screen" aria-label="Detalhes da sessão">
      <div className="head">
        <div className="grow">
          <h1 tabIndex={-1} ref={detailHeadingRef}>
            {selected.login}
          </h1>
          <div className="meta">
            Sessão PPPoE · {device.name} · {selected.state || 'ONLINE'} · IP{' '}
            {selected.address || 'Não informado nesta leitura'}
            {selected.context?.profile && ` · ${selected.context.profile}`}
            {selected.state !== 'OFFLINE' &&
              ` · conectado há ${formatPPUptime(selected.uptime)}`}
          </div>
        </div>
        <Button
          onClick={() => {
            ++detailIntentRef.current;
            select(undefined);
            p.back();
          }}
        >
          Voltar
        </Button>
      </div>
      <SessionChain
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        row={selected.row_id}
        open={openEquipment}
      />
      <SessionDetailNav detailTab={detailTab} setDetailTab={setDetailTab} />
      {detailTab === 'history' ? (
        <DataBlock
          title="Histórico de conexões"
          source="coleta local"
          data={{ state: 'unbuilt', slice: 'L-PULSO', label: 'Não feito' }}
        />
      ) : detailTab === 'traffic' ? (
        selected.state === 'OFFLINE' ? (
          <DataBlock
            title="Tráfego"
            source={`SSH (${device.name})`}
            data={{
              state: 'ready',
              content: <p>Sem leitura: sessão OFFLINE</p>,
            }}
          />
        ) : (
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
        )
      ) : detailTab === 'link' ? (
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
      ) : (
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
      )}
    </section>
  );
}
