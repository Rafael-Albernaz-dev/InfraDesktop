import { Fragment } from 'react';
import type { Credential, Device, Job } from './equipment-api';
import { equipmentAPI as api } from './equipment-api';
import { id } from './equipment-reading-format';
import { useDisplayNow } from './equipment-snapshot';
import type { EquipmentPerformFn } from './equipment-state';
import {
  formatDate,
  formatDuration,
  formatTrustOrigin,
} from '../radio/network-format';
import { Button } from '../../shared/ui/Button';
function SSHKeyDetails(p: {
  selected: Device;
  locked: boolean;
  job?: Job;
  resetOperationRef: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
}) {
  const { selected, locked, job, resetOperationRef, setResetTrust } = p;
  if (!selected.fingerprint) return null;
  return (
    <details className="det">
      <summary>Detalhes da chave SSH</summary>
      <p className="mono">{selected.fingerprint}</p>
      <p>
        {formatTrustOrigin(
          selected.trust_acceptance?.mode || selected.trust_origin,
        )}{' '}
        ·{' '}
        {selected.trust_acceptance?.accepted_at
          ? formatDate(selected.trust_acceptance.accepted_at)
          : 'Data não registrada'}
      </p>
      <Button
        disabled={locked || job?.state === 'RUNNING'}
        onClick={() => {
          resetOperationRef.current = id();
          setResetTrust(true);
        }}
      >
        Redefinir confiança
      </Button>
    </details>
  );
}
export function EquipmentAccessDetails(p: {
  selected: Device;
  credentials: Credential[];
  locked: boolean;
  job?: Job;
  detailOpen: boolean;
  resetOperationRef: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
}) {
  const { selected, credentials } = p;
  const now = useDisplayNow();
  const alias =
    credentials.find((c) => c.id === selected.credential_id)?.alias ||
    (selected.credential_id ? 'Acesso salvo' : 'Não vinculada');
  const at = selected.result?.observed_at;
  const rows = [
    ['Endereço', `${selected.address}:${selected.port}`],
    ['Credencial', alias],
    [
      'Última leitura válida',
      at
        ? 'há ' + formatDuration((now - Date.parse(at)) / 1000)
        : 'Ainda sem leitura',
    ],
    [
      'Identidade SSH',
      selected.fingerprint ? 'Chave salva' : 'Primeira conexão pendente',
    ],
  ];
  return (
    <>
      <dl className="kv">
        {rows.map(([key, value]) => (
          <Fragment key={key}>
            <dt>{key}</dt>
            <dd>{value}</dd>
          </Fragment>
        ))}
      </dl>
      <SSHKeyDetails {...p} />
    </>
  );
}
export function EquipmentSecondaryActions(p: {
  selected: Device;
  connected: boolean;
  locked: boolean;
  job?: Job;
  profile: string;
  activation: string;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
  setError: (v: string) => void;
  setForm: (d: Device | undefined) => void;
  setSelected: (d: Device | undefined) => void;
  perform: EquipmentPerformFn;
}) {
  const { selected } = p;
  return (
    <>
      <Button
        label="Editar equipamento"
        disabled={!p.connected}
        onClick={() => {
          p.setError('');
          p.setForm(selected);
        }}
      >
        Editar
      </Button>
      <Button
        disabled={p.locked || !selected.active || p.job?.state === 'RUNNING'}
        onClick={() => void p.startOperation('inspect', selected)}
      >
        Verificar identidade
      </Button>
      <Button
        disabled={p.locked}
        onClick={() =>
          void p.perform(
            () =>
              api.active(
                p.profile,
                p.activation,
                selected.id,
                id(),
                selected.revision,
                !selected.active,
              ),
            (r) => {
              if (r.data?.device) p.setSelected(r.data.device);
            },
          )
        }
      >
        {selected.active ? 'Desativar' : 'Reativar'}
      </Button>
    </>
  );
}
