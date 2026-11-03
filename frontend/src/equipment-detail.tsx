import { Button } from './desktop';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Credential, Device, Job } from './equipment-api';
import type { EquipmentPerformFn } from './equipment-dialogs';
import { EquipmentInsights } from './equipment-insights';
import { EquipmentReading, id, stamp } from './equipment-reading';
import { formatDate, formatTrustOrigin } from './network-format';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { RadioEquipment } from './radio';
import { RadioRoles } from './radio-contract';

const ROLE_LABELS: Record<string, string> = {
  concentrator: 'Concentrador',
  [RadioRoles.ap]: 'AP',
  [RadioRoles.cpe]: 'CPE',
  [RadioRoles.ptp]: 'PTP',
  other: 'Outro',
};

function SSHKeyDetails(p: {
  selected: Device;
  locked: boolean;
  job?: Job;
  resetOperation: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
}) {
  const { selected, locked, job } = p;
  if (!selected.fingerprint) return null;
  return (
    <details className="equipment-help">
      <summary>Detalhes da chave SSH</summary>
      <p className="fingerprint mono">{selected.fingerprint}</p>
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
          p.resetOperation.current = id();
          p.setResetTrust(true);
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
  resetOperation: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
}) {
  const { selected, credentials, locked, job, detailOpen } = p;
  const alias =
    credentials.find((c) => c.id === selected.credential_id)?.alias ||
    (selected.credential_id ? 'Acesso salvo' : 'Não vinculada');
  return (
    <>
      <div className="kv">
        <span className="key">Endereço</span>
        <span className="mono">
          {selected.address}:{selected.port}
        </span>
      </div>
      <div className="kv">
        <span className="key">Credencial de consulta</span>
        <span>{alias}</span>
      </div>
      <div className="kv">
        <span className="key">Última leitura válida</span>
        <span>{stamp(selected)}</span>
      </div>
      <div className="kv">
        <span className="key">Identidade do destino</span>
        <span>
          {selected.fingerprint ? 'Chave salva' : 'Primeira conexão pendente'}
        </span>
      </div>
      <SSHKeyDetails {...p} />
      {detailOpen && (
        <Button
          disabled={locked || !selected.active || job?.state === 'RUNNING'}
          onClick={() => void p.startOperation('inspect', selected)}
        >
          Verificar identidade
        </Button>
      )}
    </>
  );
}

function PrimaryQueryButton(p: {
  selected: Device;
  detailOpen: boolean;
  locked: boolean;
  vault: boolean;
  job?: Job;
  setDetailOpen: (v: boolean) => void;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
}) {
  const { selected, detailOpen, locked, vault, job } = p;
  if (selected.role === RadioRoles.ptp) return null;
  if (
    (selected.role === RadioRoles.ap || selected.role === RadioRoles.cpe) &&
    selected.fingerprint
  ) {
    if (detailOpen) return null;
    return (
      <Button kind="primary" onClick={() => p.setDetailOpen(true)}>
        Abrir rádio
      </Button>
    );
  }
  return (
    <Button
      kind={
        detailOpen && selected.role !== 'concentrator' ? 'primary' : undefined
      }
      disabled={
        locked ||
        !vault ||
        !selected.active ||
        !selected.credential_id ||
        job?.state === 'RUNNING'
      }
      onClick={() => {
        p.setDetailOpen(true);
        void p.startOperation(
          selected.fingerprint ? 'query' : 'inspect',
          selected,
        );
      }}
    >
      {detailOpen
        ? selected.result
          ? 'Atualizar resumo'
          : 'Consultar resumo'
        : selected.result
          ? 'Atualizar'
          : 'Conectar'}
    </Button>
  );
}

function EquipmentSecondaryActions(p: {
  selected: Device;
  detailOpen: boolean;
  connected: boolean;
  locked: boolean;
  job?: Job;
  profile: string;
  activation: string;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
  openDevice: (d: Device) => void;
  setError: (v: string) => void;
  setForm: (d: Device | 'new' | undefined) => void;
  setSelected: (d: Device | undefined) => void;
  perform: EquipmentPerformFn;
}) {
  const { selected, detailOpen, connected, locked } = p;
  return (
    <>
      {!detailOpen && selected.role !== RadioRoles.ptp && (
        <Button
          disabled={locked || !selected.active || p.job?.state === 'RUNNING'}
          onClick={() => void p.startOperation('inspect', selected)}
        >
          Verificar identidade
        </Button>
      )}
      {!detailOpen && (
        <Button
          kind="flat"
          disabled={locked}
          onClick={() => p.openDevice(selected)}
        >
          Abrir equipamento
        </Button>
      )}
      <Button
        kind="flat"
        disabled={!connected}
        onClick={() => {
          p.setError('');
          p.setForm(selected);
        }}
      >
        Editar equipamento
      </Button>
      <Button
        kind="flat"
        disabled={locked}
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

function EquipmentActionsBar(p: {
  selected: Device;
  detailOpen: boolean;
  connected: boolean;
  locked: boolean;
  vault: boolean;
  job?: Job;
  profile: string;
  activation: string;
  setDetailOpen: (v: boolean) => void;
  setPPOpen: (v: boolean) => void;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
  openDevice: (d: Device) => void;
  setError: (v: string) => void;
  setForm: (d: Device | 'new' | undefined) => void;
  setSelected: (d: Device | undefined) => void;
  perform: EquipmentPerformFn;
}) {
  const { selected, detailOpen, connected } = p;
  return (
    <div className="bottom-actions equipment-actions">
      {selected.role === 'concentrator' &&
        (!detailOpen || !selected.result) && (
          <Button
            kind={detailOpen ? 'primary' : undefined}
            disabled={!connected || !selected.fingerprint}
            onClick={() => {
              p.setDetailOpen(true);
              p.setPPOpen(true);
            }}
          >
            Abrir sessões PPPoE
          </Button>
        )}
      <PrimaryQueryButton {...p} />
      <EquipmentSecondaryActions {...p} />
    </div>
  );
}

function EquipmentExtendedPanels(p: {
  selected: Device;
  detailOpen: boolean;
  profile: string;
  activation: string;
  locked: boolean;
  vault: boolean;
  setPPOpen: (v: boolean) => void;
}) {
  const { selected, detailOpen, profile, activation, locked, vault } = p;
  const isRadio =
    selected.role === RadioRoles.ap ||
    selected.role === RadioRoles.cpe ||
    selected.role === RadioRoles.ptp;
  const blocked = locked || !vault || !selected.active || !selected.fingerprint;
  return (
    <>
      {selected.result && (
        <EquipmentReading
          key={selected.id + selected.result.observed_at}
          device={selected}
          focus={false}
          separate={detailOpen}
          openPPPoE={
            selected.role === 'concentrator'
              ? () => p.setPPOpen(true)
              : undefined
          }
        />
      )}
      {detailOpen && selected.role === 'concentrator' && (
        <>
          <EquipmentInsights
            profile={profile}
            activation={activation}
            device={selected}
            blocked={blocked}
          />
          <PPPoEReadPanel
            profile={profile}
            activation={activation}
            device={selected}
            observation=""
            rows={[]}
            health
            blocked={blocked}
          />
        </>
      )}
      {detailOpen && isRadio && (
        <RadioEquipment
          key={[profile, activation, selected.id, selected.revision].join('|')}
          profile={profile}
          activation={activation}
          device={selected}
          blocked={locked || !vault || !selected.active}
        />
      )}
    </>
  );
}

function EquipmentDetailHeader(p: {
  selected: Device;
  detailOpen: boolean;
  surface: React.MutableRefObject<number>;
  detailRead: React.MutableRefObject<number>;
  setDetailOpen: (v: boolean) => void;
  setJob: (j: Job | undefined) => void;
}) {
  const { selected, detailOpen } = p;
  return (
    <>
      {detailOpen && (
        <Button
          kind="flat"
          onClick={() => {
            p.surface.current++;
            p.detailRead.current++;
            p.setDetailOpen(false);
            p.setJob(undefined);
          }}
        >
          Voltar aos equipamentos
        </Button>
      )}
      <p className="inspector-heading">Detalhes do equipamento</p>
      <h2>{selected.name}</h2>
      <p>Papel declarado: {ROLE_LABELS[selected.role || 'other']}</p>
      {(!selected.role || selected.role === 'other') && (
        <p>
          Para ver os logins de clientes PPPoE, edite o cadastro e declare o
          papel Concentrador.
        </p>
      )}
      <p className="subtitle">
        SSH · {selected.result?.family || 'Sistema desconhecido'}
      </p>
    </>
  );
}

export function EquipmentDetailPane(p: {
  selected: Device;
  detailOpen: boolean;
  detailSurface: React.RefObject<HTMLElement | null>;
  surface: React.MutableRefObject<number>;
  detailRead: React.MutableRefObject<number>;
  setDetailOpen: (v: boolean) => void;
  setJob: (j: Job | undefined) => void;
  setPPOpen: (v: boolean) => void;
  credentials: Credential[];
  connected: boolean;
  locked: boolean;
  vault: boolean;
  job?: Job;
  profile: string;
  activation: string;
  resetOperation: React.MutableRefObject<string>;
  setResetTrust: (v: boolean) => void;
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>;
  openDevice: (d: Device) => void;
  setError: (v: string) => void;
  setForm: (d: Device | 'new' | undefined) => void;
  setSelected: (d: Device | undefined) => void;
  perform: EquipmentPerformFn;
}) {
  const { selected, detailOpen } = p;
  const accessDetails = <EquipmentAccessDetails {...p} />;
  return (
    <aside
      ref={p.detailSurface}
      tabIndex={-1}
      className={detailOpen ? 'equipment-detail' : 'inspector'}
    >
      <EquipmentDetailHeader {...p} />
      {!detailOpen && accessDetails}
      <EquipmentActionsBar {...p} />
      {selected.last_attempt && (
        <p className="inventory-notice" role="status">
          Última tentativa: {formatDate(selected.last_attempt.at)} ·{' '}
          {selected.last_attempt.state === 'SUCCEEDED'
            ? 'Leitura concluída'
            : selected.last_attempt.state === 'STARTED'
              ? 'Iniciada; conclusão ainda não registrada'
              : equipmentMessage(selected.last_attempt.code)}
        </p>
      )}
      <EquipmentExtendedPanels {...p} />
      {detailOpen && (
        <details className="equipment-access">
          <summary>Acesso e identidade</summary>
          {accessDetails}
        </details>
      )}
    </aside>
  );
}
