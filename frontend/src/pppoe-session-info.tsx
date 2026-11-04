import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';
import type { Device } from './equipment-api';
import { uptimeTitle } from './network-format';
import { OfflineEvidence, SessionContext } from './pppoe-context';
import { PPPoEPing } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import type { PPInterval, PPObservation, PPRow } from './presentation';

export function SessionAuthInfo(p: {
  selected: PPRow;
  current: PPObservation;
  fastTrack?: number;
}) {
  const { selected, current } = p;
  return (
    <DataBlock
      title="Informações"
      source="SSH"
      age={readingAge(current.provenance.ended_at)}
      data={{
        state: 'ready',
        content: (
          <>
            <p>IP: {selected.address || 'Indisponível'}</p>
            <p>Caller ID: {selected.caller_id || 'Indisponível'}</p>
            <p title={uptimeTitle(selected.uptime)}>
              {selected.state === 'OFFLINE'
                ? 'Duração da última sessão: '
                : 'Tempo conectado: '}
              {formatPPUptime(selected.uptime)}
            </p>
            <p>Estado: {selected.state || 'ONLINE'}</p>
            <SessionContext
              row={selected}
              omitted={current.context_omitted}
              fastTrack={p.fastTrack}
            />
            <p>ID no roteador: {selected.remote_id || 'Indisponível'}</p>
            <p>Session ID: {selected.session_id || 'Indisponível'}</p>
            {selected.state === 'OFFLINE' && <OfflineEvidence row={selected} />}
          </>
        ),
      }}
    />
  );
}
export function SessionConnectivityTab(p: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  uptime: PPInterval;
  disabled: boolean;
  isOffline: boolean;
  openEquipment?: (id: string) => void;
}) {
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
