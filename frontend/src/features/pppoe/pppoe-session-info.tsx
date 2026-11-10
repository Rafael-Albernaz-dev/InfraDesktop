import type { Device } from '../equipment/equipment-api';
import { readingAge } from '../equipment/equipment-reading-format';
import { PPPoEPing } from './pppoe-diagnostics';
import type { SessionAuthInfoProps } from './pppoe-session-auth-content';
import { SessionAuthContent } from './pppoe-session-auth-content';
import type { PPInterval } from '../../platform/wails/pppoe-types';
import { DataBlock } from '../../shared/ui/DataBlock';

export function SessionAuthInfo(p: SessionAuthInfoProps) {
  const { current } = p;
  return (
    <DataBlock
      title="Informações"
      className="session-info-block"
      source={`SSH (${p.device.name})`}
      age={readingAge(current.provenance.ended_at)}
      data={{
        state: 'ready',
        content: (
          <>
            <SessionAuthContent {...p} />
          </>
        ),
      }}
    />
  );
}
type SessionConnectivityTabProps = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  uptime: PPInterval;
  disabled: boolean;
  isOffline: boolean;
  openEquipment?: (id: string) => void;
};
export function SessionConnectivityTab(p: SessionConnectivityTabProps) {
  return (
    <DataBlock
      title="Ping pelo concentrador"
      source={`SSH (${p.device.name})`}
      data={{
        state: 'ready',
        content: (
          <>
            <PPPoEPing
              key={p.observation + '|' + p.row}
              profile={p.profile}
              activation={p.activation}
              device={p.device}
              compact
              observation={p.observation}
              row={p.row}
              blocked={p.disabled || p.isOffline}
            />
            {p.isOffline && <p>Sem leitura: sessão OFFLINE</p>}
          </>
        ),
      }}
    />
  );
}
