import { EquipmentPortHistory } from './equipment-port-history';
import { DataBlock } from '../../shared/ui/DataBlock';
import { KeyValues } from '../../shared/ui/KeyValues';
import {
  DeviceHealthSection,
  DeviceOperationalTables,
} from '../monitoring/device-operational';
import { usePPReadState } from '../pppoe/pppoe-read-state';
import {
  PPReadActionButtons,
  PPReadStatusBanners,
} from '../pppoe/pppoe-read-views';
import { PPReadResultPagination } from '../pppoe/pppoe-read-result';
import { readingAge, formatRestarts } from './equipment-reading-format';
import { formatClockOffset, formatDuration } from './network-format';
import { EquipmentPorts } from './equipment-health-tables';
import { Button } from '../../shared/ui/Button';
import { useEffect, useState, useCallback } from 'react';
import { deviceMonitoringAPI } from '../../platform/wails/presentation';
import type { Device, PPReadRow } from '../../platform/wails/presentation';
import { useDisplayNow, useEquipmentSnapshot } from './equipment-snapshot';
import { PPPoEReadPanel } from '../pppoe/pppoe-diagnostics';
import { formatDate } from './network-format';
export function EquipmentInsights(p: {
  profile: string;
  activation: string;
  device: Device;
  blocked?: boolean;
}) {
  const { profile, activation, device, blocked = false } = p;
  const displayNow = useDisplayNow();
  const [refresh, bump] = useState(0),
    [historyOpen, showHistory] = useState(false);
  const [history, setHistory] = useState<{
    key: string;
    rows: PPReadRow[];
    next: string;
    error: boolean;
  }>();
  const [cursor, setCursor] = useState('');
  const key = [
    profile,
    activation,
    device.id,
    device.revision,
    refresh,
    cursor,
  ].join('|');
  const snapshot = useEquipmentSnapshot(
    profile,
    activation,
    device.id,
    device.revision,
    refresh,
  );
  useEffect(() => {
    if (!historyOpen) return;
    let alive = true;
    void deviceMonitoringAPI
      .current(profile, activation, device.id, 'port-history', cursor)
      .then((r) => {
        if (!alive) return;
        const valid =
          r.profile_id === profile &&
          r.activation_id === activation &&
          r.code === 'OK';
        setHistory({
          key,
          rows: valid ? r.data?.read?.rows || [] : [],
          next: valid ? r.data?.next_cursor || '' : '',
          error: !valid,
        });
      })
      .catch(() => {
        if (alive) setHistory({ key, rows: [], next: '', error: true });
      });
    return () => {
      alive = false;
    };
  }, [
    profile,
    activation,
    device.id,
    device.revision,
    historyOpen,
    cursor,
    refresh,
    key,
  ]);
  const completed = useCallback(() => bump((n) => n + 1), []);
  const readProps = {
    profile,
    activation,
    device,
    observation: '',
    rows: [],
    monitoring: true,
    summaryOnly: true,
    blocked,
    onRead: completed,
  };
  const read = usePPReadState(readProps);
  const resource = snapshot?.rows.find((r) => r.kind === 'resource');
  const health = resource?.health,
    restarts = snapshot?.rows.find((r) => r.kind === 'restarts');
  const ports = snapshot?.rows.filter((r) => r.kind === 'port') || [];
  const currentHistory = history?.key === key ? history : undefined;
  const snapshotLimited = snapshot?.rows.some(
    (r) =>
      r.kind === 'availability' &&
      r.name === 'retention' &&
      r.value === 'limited',
  );
  const historyLimited =
    snapshotLimited ||
    currentHistory?.rows.some(
      (r) =>
        r.kind === 'availability' &&
        r.name === 'retention' &&
        r.value === 'limited',
    );
  const healthRows: [string, React.ReactNode][] = [
    [
      'Reinícios 7 d',
      <span key="restarts" title={formatRestarts(restarts)}>
        {restarts?.used ?? 'Não informado nesta leitura'}
      </span>,
    ],
    [
      'Regras FastTrack habilitadas',
      health?.fasttrack_rules ?? 'Não informado nesta leitura',
    ],
    [
      'Relógio roteador × PC',
      health?.clock_offset.known
        ? health.clock_offset.min === health.clock_offset.max
          ? formatDuration(health.clock_offset.min / 1e9)
          : formatClockOffset(health.clock_offset)
        : 'Não informado nesta leitura',
    ],
    [
      'NTP',
      <span key="ntp">
        {health?.ntp_enabled === undefined
          ? 'Não informado nesta leitura'
          : health.ntp_enabled
            ? 'Ativado'
            : 'Desativado'}
        {health && (
          <details className="inline-details">
            <summary>Detalhes</summary>
            <DeviceHealthSection health={health} showFastTrackNotice={false} />
          </details>
        )}
      </span>,
    ],
  ];
  return (
    <section aria-label="Saúde e portas do equipamento">
      <div className="bar">
        <span className="muted">
          {resource?.at
            ? `Snapshot salvo de ${formatDate(resource.at)}.`
            : 'Ainda sem leitura salva.'}
        </span>
        <span className="grow" />
        <PPReadActionButtons s={read} pProps={readProps} />
        {!historyOpen && (
          <Button
            aria-expanded={historyOpen}
            onClick={() => {
              showHistory((v) => !v);
              setCursor('');
            }}
          >
            Histórico de portas
          </Button>
        )}
      </div>
      <PPReadStatusBanners
        s={{ ...read, firstStatus: undefined }}
        pProps={readProps}
      />
      {snapshot?.failed && (
        <p role="alert">Leitura salva do equipamento indisponível.</p>
      )}
      {health?.fasttrack_rules !== undefined && health.fasttrack_rules > 0 && (
        <p role="status" className="inventory-notice">
          Sem controle de banda: FastTrack ativo
        </p>
      )}
      {snapshotLimited && (
        <p role="status">
          Histórico parcial: parte dos registros foi removida pelo limite de
          armazenamento. Contagens de reinícios consideram apenas os registros
          retidos.
        </p>
      )}
      <div className="grid">
        <DataBlock
          title="Saúde"
          source="SSH"
          age={readingAge(resource?.at, displayNow)}
          data={
            !snapshot
              ? { state: 'loading' }
              : { state: 'ready', content: <KeyValues rows={healthRows} /> }
          }
        />
        <DataBlock
          title="Portas físicas negociadas"
          source="SSH"
          age={readingAge(resource?.at, displayNow)}
          data={
            ports.length
              ? {
                  state: 'ready',
                  content: <EquipmentPorts ports={ports} compact />,
                }
              : {
                  state: 'empty',
                  message: 'Nenhuma leitura salva de portas físicas.',
                }
          }
        />
        <PPPoEReadPanel
          profile={profile}
          activation={activation}
          device={device}
          observation=""
          rows={[]}
          health
          healthView="pools"
          blocked={blocked}
        />
      </div>
      <div className="sp10" />
      <DataBlock
        title="Leitura operacional"
        source="SSH"
        age={readingAge(read.result?.ended_at)}
        data={{
          state: 'ready',
          content: (
            <>
              {read.result ? (
                <DeviceOperationalTables result={read.result} />
              ) : (
                <p>Ainda sem leitura</p>
              )}
              {read.result && (
                <PPReadResultPagination s={read} pProps={readProps} />
              )}
            </>
          ),
        }}
      />
      {historyOpen && (
        <EquipmentPortHistory
          read={currentHistory}
          limited={historyLimited}
          at={resource?.at}
          now={displayNow}
          cursor={cursor}
          setCursor={setCursor}
          close={() => showHistory(false)}
        />
      )}
    </section>
  );
}
