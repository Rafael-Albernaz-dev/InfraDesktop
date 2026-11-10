import type { PPRow } from '../../platform/wails/presentation';
import type { Device } from '../equipment/equipment-api';
import {
  exactRate,
  formatRate,
  uptimeTitle,
} from '../equipment/network-format';
import { OfflineEvidence, SessionContext } from './pppoe-context';
import { formatPPUptime } from './pppoe-format';
import type { PPObservation } from '../../platform/wails/presentation-pppoe-types';
import { SessionCPE } from './session-cpe';
import { KeyValues } from '../../shared/ui/KeyValues';

export type SessionAuthInfoProps = {
  selected: PPRow;
  current: PPObservation;
  profile: string;
  activation: string;
  device: Device;
  fastTrack?: number;
  openEquipment?: (id: string) => void;
};
const known = 'Não informado nesta leitura';
type InformationRows = Parameters<typeof KeyValues>[0]['rows'];
function authRows(p: SessionAuthInfoProps): InformationRows {
  const { selected, current } = p;
  const c = selected.context,
    associated = c?.association === 'associated';
  return [
    [
      'Autenticação',
      <details>
        <summary>{known} · Mais informações</summary>
        <SessionContext
          row={selected}
          omitted={current.context_omitted}
          fastTrack={p.fastTrack}
        />
        {selected.state === 'OFFLINE' && <OfflineEvidence row={selected} />}
      </details>,
    ],
    [
      'Plano',
      associated && c?.secret_state === 'active' ? c.profile || known : known,
    ],
  ];
}
function planRows(p: SessionAuthInfoProps): InformationRows {
  const { selected } = p;
  const c = selected.context,
    associated = c?.association === 'associated';
  const limits =
    associated &&
    c?.download_limit !== undefined &&
    c.upload_limit !== undefined
      ? `${formatRate(c.download_limit)} / ${formatRate(c.upload_limit)}`
      : known;
  return [
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
          <small className="cell-sub">limite lido, não garantido</small>
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
  ];
}
function identityRows(p: SessionAuthInfoProps): InformationRows {
  const { selected } = p;
  const c = selected.context,
    associated = c?.association === 'associated';
  return [
    ['Caller ID', selected.caller_id || known],
    [
      'VLAN / serviço / interface',
      associated
        ? `${c?.vlan ?? known} / ${c?.service || known} / ${c?.interface || known}`
        : known,
    ],
    ['Duplicidade', duplicationLabel(selected.duplicates)],
  ];
}
function sessionRows(p: SessionAuthInfoProps): InformationRows {
  const { selected, current } = p;
  return [
    [
      'Duração da última sessão',
      selected.state === 'OFFLINE' ? formatPPUptime(selected.uptime) : known,
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
  ];
}

export function SessionAuthContent(p: SessionAuthInfoProps) {
  return (
    <KeyValues
      rows={[
        ...authRows(p),
        ...planRows(p),
        ...identityRows(p),
        ...sessionRows(p),
      ]}
    />
  );
}
function duplicationLabel(count: number) {
  if (count > 1) return `${count} sessões`;
  if (count === 1) return 'Sem duplicidade';
  return known;
}
