import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';
import type { Device } from './equipment-api';
import { exactRate, formatRate, uptimeTitle } from './network-format';
import { KeyValues } from './ui/KeyValues';
import { SessionCPE } from './session-cpe';
import { OfflineEvidence, SessionContext } from './pppoe-context';
import { PPPoEPing } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import type { PPInterval, PPObservation, PPRow } from './presentation';

export function SessionAuthInfo(p: {
  selected: PPRow;
  current: PPObservation;
  profile: string;
  activation: string;
  device: Device;
  fastTrack?: number;
  openEquipment?: (id: string) => void;
}) {
  const { selected, current } = p;
  const c = selected.context;
  const associated = c?.association === 'associated';
  const known = 'Não informado nesta leitura';
  const limits =
    associated &&
    c?.download_limit !== undefined &&
    c.upload_limit !== undefined
      ? `${formatRate(c.download_limit)} / ${formatRate(c.upload_limit)}`
      : known;
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
            <KeyValues
              rows={[
                [
                  'Autenticação',
                  <details>
                    <summary>{known} · Mais informações</summary>
                    <SessionContext
                      row={selected}
                      omitted={current.context_omitted}
                      fastTrack={p.fastTrack}
                    />
                    {selected.state === 'OFFLINE' && (
                      <OfflineEvidence row={selected} />
                    )}
                  </details>,
                ],
                [
                  'Plano',
                  associated && c?.secret_state === 'active'
                    ? c.profile || known
                    : known,
                ],
                [
                  'Limite de plano',
                  <span
                    title={
                      associated && c
                        ? `${exactRate(c.download_limit)} download / ${exactRate(c.upload_limit)} upload`
                        : undefined
                    }
                  >
                    {limits}
                    {p.fastTrack !== undefined && p.fastTrack > 0 && (
                      <small className="cell-sub">
                        limite lido, não garantido
                      </small>
                    )}
                  </span>,
                ],
                [
                  'Conectividade',
                  <span title={uptimeTitle(selected.uptime)}>
                    IP {selected.address || known}
                    {selected.state === 'OFFLINE'
                      ? ' · OFFLINE'
                      : ` · há ${formatPPUptime(selected.uptime)}`}
                  </span>,
                ],
                ['Caller ID', selected.caller_id || known],
                [
                  'VLAN / serviço / interface',
                  associated
                    ? `${c?.vlan ?? known} / ${c?.service || known} / ${c?.interface || known}`
                    : known,
                ],
                [
                  'Duplicidade',
                  selected.duplicates > 1
                    ? `${selected.duplicates} sessões`
                    : selected.duplicates === 1
                      ? 'Sem duplicidade'
                      : known,
                ],
                [
                  'Duração da última sessão',
                  selected.state === 'OFFLINE'
                    ? formatPPUptime(selected.uptime)
                    : known,
                ],
                ['ID no roteador', selected.remote_id || known],
                ['Session ID', selected.session_id || known],
                [
                  'Vínculo com CPE',
                  <SessionCPE
                    profile={p.profile}
                    activation={p.activation}
                    deviceId={p.device.id}
                    observation={current.observation_id}
                    row={selected.row_id}
                    open={p.openEquipment}
                  />,
                ],
              ]}
            />
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
