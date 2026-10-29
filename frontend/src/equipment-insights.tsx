import { useEffect, useState } from 'react';
import { deviceMonitoringAPI } from './presentation';
import type { Device, PPReadRow } from './presentation';
import { useDisplayNow, useEquipmentSnapshot } from './equipment-snapshot';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import {
  exactNumber,
  exactRate,
  formatDate,
  formatDuration,
  portLabel,
} from './network-format';

export function EquipmentInsights({
  profile,
  activation,
  device,
  blocked = false,
}: {
  profile: string;
  activation: string;
  device: Device;
  blocked?: boolean;
}) {
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
  const historyChanges =
    currentHistory?.rows.filter((r) => r.kind === 'port_change') || [];
  return (
    <section
      className="pppoe-diagnostics"
      aria-label="Saúde e portas do equipamento"
    >
      <h2>Saúde e portas</h2>
      <PPPoEReadPanel
        profile={profile}
        activation={activation}
        device={device}
        observation=""
        rows={[]}
        monitoring
        summaryOnly
        blocked={blocked}
        onRead={() => bump((n) => n + 1)}
      />
      {snapshot?.failed && (
        <p role="alert">Leitura salva do equipamento indisponível.</p>
      )}
      {resource?.at && (
        <p title={resource.at}>
          Snapshot salvo: {formatDate(resource.at)} · há{' '}
          {formatDuration((displayNow - Date.parse(resource.at)) / 1000)}.
          Últimos dados conhecidos.
        </p>
      )}
      {health?.fasttrack_rules !== undefined && health.fasttrack_rules > 0 && (
        <p role="status" className="inventory-notice">
          Sem controle de banda: FastTrack ativo
        </p>
      )}
      {health && (
        <p>
          Regras FastTrack habilitadas: {exactNumber(health.fasttrack_rules)}.
        </p>
      )}
      {restarts && (
        <p>
          Reinícios 7 d: {exactNumber(restarts.used)} · 30 d:{' '}
          {exactNumber(restarts.size)}. Detectados entre leituras; reinícios
          múltiplos no mesmo intervalo contam como um.
        </p>
      )}
      {snapshotLimited && (
        <p role="status">
          Histórico parcial: parte dos registros foi removida pelo limite de
          armazenamento. Contagens de reinícios consideram apenas os registros
          retidos.
        </p>
      )}
      {!snapshot ? (
        <p role="status">Lendo snapshot salvo…</p>
      ) : (
        !snapshot.rows.length &&
        !snapshot.failed && (
          <p>
            Nenhuma leitura salva de saúde e portas. Atualize o monitoramento.
          </p>
        )
      )}
      <p>
        Velocidade negociada de portas físicas ether. Interfaces não físicas:
        não se aplica.
      </p>
      {!!ports.length && (
        <div
          className="table-scroll"
          tabIndex={0}
          aria-label="Portas físicas negociadas"
        >
          <table className="app-table">
            <thead>
              <tr>
                <th>Porta</th>
                <th>Velocidade / duplex</th>
                <th>Quedas acumuladas</th>
                <th>Leitura</th>
              </tr>
            </thead>
            <tbody>
              {ports.map((r) => (
                <tr key={r.name}>
                  <td>{r.name}</td>
                  <td title={exactRate(r.port?.speed_bps)}>
                    {portLabel(r.port)}
                    {r.port?.running &&
                    ((r.port.speed_bps !== undefined &&
                      r.port.speed_bps <= 100000000) ||
                      r.port.full_duplex === false)
                      ? ' · atenção'
                      : ''}
                  </td>
                  <td>{exactNumber(r.port?.link_downs)}</td>
                  <td title={r.at}>{formatDate(r.at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <button
        aria-expanded={historyOpen}
        onClick={() => {
          showHistory((v) => !v);
          setCursor('');
        }}
      >
        Histórico de portas
      </button>
      {historyOpen && (
        <section aria-label="Histórico de portas salvo">
          <h3>Histórico de portas</h3>
          <p>
            Mudanças entre leituras salvas; janela de 30 dias. Abrir este
            histórico não consulta o roteador.
          </p>
          {!currentHistory ? (
            <p role="status">Lendo histórico salvo…</p>
          ) : currentHistory.error ? (
            <p role="alert">Histórico indisponível.</p>
          ) : !historyChanges.length ? (
            <p>
              {historyLimited
                ? 'Sem eventos de porta entre os registros retidos; histórico parcial.'
                : 'Nenhuma mudança de porta detectada entre as leituras disponíveis.'}
            </p>
          ) : (
            <div className="table-scroll" tabIndex={0}>
              <table className="app-table">
                <thead>
                  <tr>
                    <th>Quando</th>
                    <th>Porta</th>
                    <th>De → Para</th>
                  </tr>
                </thead>
                <tbody>
                  {historyChanges.map((r, i) => (
                    <tr key={i}>
                      <td title={r.at}>{formatDate(r.at, true)}</td>
                      <td>{r.name}</td>
                      <td>
                        {portLabel(r.before_port)} (
                        {exactNumber(r.before_port?.link_downs)} quedas) →{' '}
                        {portLabel(r.port)} ({exactNumber(r.port?.link_downs)}{' '}
                        quedas)
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {currentHistory?.next && (
            <button onClick={() => setCursor(currentHistory.next)}>
              Próxima página do histórico de portas
            </button>
          )}
          {cursor && (
            <button onClick={() => setCursor('')}>
              Início do histórico de portas
            </button>
          )}
        </section>
      )}
    </section>
  );
}
