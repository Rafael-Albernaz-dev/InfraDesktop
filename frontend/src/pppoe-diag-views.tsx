import { useState } from 'react';
import type { Device } from './equipment-api';
import {
  exactNumber,
  formatDate,
  formatLatency,
  formatLatencyRange,
  formatMetric,
  uptimeTitle,
} from './network-format';
import type { PDRow } from './presentation';
import { diagnosticMessage, useDiagnostic } from './pppoe-diag-state';

const state = (value?: boolean) =>
  value === undefined ? 'Indisponível' : value ? 'Sim' : 'Não';
const relation: Record<string, string> = {
  resolved: 'Pai identificado',
  missing: 'Pai ausente',
  ambiguous: 'Vínculo ambíguo',
  cycle: 'Ciclo na configuração',
  depth_limited: 'Encadeamento excede o limite',
};

function RadiusCounters({
  counters,
}: {
  counters: NonNullable<PDRow['counters']>;
}) {
  const fields = {
    Pendentes: counters.pending,
    Solicitações: counters.requests,
    Aceitas: counters.accepts,
    Rejeitadas: counters.rejects,
    Reenvios: counters.resends,
    Timeouts: counters.timeouts,
    'Respostas inválidas': counters.bad_replies,
  };
  return (
    <dl className="pppoe-diagnostic-values">
      {Object.entries(fields).map(([name, value]) => (
        <div key={name}>
          <dt>{name}</dt>
          <dd>{exactNumber(value)}</dd>
        </div>
      ))}
      <div>
        <dt>RTT da última solicitação</dt>
        <dd title={uptimeTitle(counters.last_rtt)}>
          {counters.last_rtt.known
            ? formatLatencyRange(counters.last_rtt)
            : 'Indisponível'}
        </dd>
      </div>
    </dl>
  );
}

function DiagnosticRadiusRow({ row }: { row: PDRow }) {
  return (
    <article className="pppoe-diagnostic-row">
      <h3>{row.address}</h3>
      <p>
        Serviços: {row.services} · Protocolo:{' '}
        {row.protocol === '?' ? 'Indisponível' : row.protocol} · Desativado:{' '}
        {state(row.disabled)}
      </p>
      <p>
        Autenticação: porta {row.auth_port} · Accounting: porta{' '}
        {row.accounting_port}
      </p>
      {row.counters && <RadiusCounters counters={row.counters} />}
    </article>
  );
}

function DiagnosticServiceRow({ row }: { row: PDRow }) {
  const kindLabel =
    row.kind === 'service'
      ? 'Serviço PPPoE'
      : row.kind === 'vlan'
        ? 'VLAN configurada'
        : `Interface ${row.type || ''}`;
  return (
    <article className="pppoe-diagnostic-row">
      <h3>{row.name || 'Serviço sem nome'}</h3>
      <p>
        {kindLabel} · Desativado: {state(row.disabled)}
        {row.running !== undefined
          ? ` · Em execução: ${state(row.running)}`
          : ''}
        {row.dynamic !== undefined ? ` · Dinâmica: ${state(row.dynamic)}` : ''}
      </p>
      {row.parent !== undefined && (
        <p>
          Interface pai: {row.parent} ·{' '}
          {relation[row.relation || ''] || 'Vínculo indisponível'}
        </p>
      )}
      {row.kind === 'vlan' && (
        <p>
          Tag: {row.vlan_id} · Service tag: {state(row.service_tag)}.{' '}
          {row.relation === 'resolved'
            ? 'A relação representa a configuração, sem comprovar tráfego individual.'
            : 'O vínculo não pôde ser comprovado.'}
        </p>
      )}
      {row.kind === 'service' && (
        <p>
          Faixas de VLAN:{' '}
          {row.range === '?'
            ? 'Indisponível neste firmware/permissão'
            : row.range || 'Sem faixa configurada'}{' '}
          · Aceita sem tag: {state(row.accept_untagged)}
        </p>
      )}
    </article>
  );
}

export function DiagnosticRow({ row }: { row: PDRow }) {
  if (row.not_seen_since) {
    return (
      <article className="pppoe-diagnostic-row">
        <h3>{row.name || 'Interface sem nome'}</h3>
        <p title={row.not_seen_since}>
          Não visto desde {formatDate(row.not_seen_since, true)}. Estado atual
          indisponível; última interface dinâmica observada.
        </p>
      </article>
    );
  }
  if (row.kind === 'aaa') {
    return (
      <article className="pppoe-diagnostic-row">
        <h3>PPP AAA</h3>
        <p>
          Usa RADIUS: {state(row.use_radius)} · Accounting:{' '}
          {state(row.accounting)}
        </p>
      </article>
    );
  }
  if (row.kind === 'radius') return <DiagnosticRadiusRow row={row} />;
  return <DiagnosticServiceRow row={row} />;
}

type DiagView = ReturnType<typeof useDiagnostic>;

function DiagnosticListContent(p: {
  view: DiagView;
  scope: 'catalog' | 'radius';
  kind: string;
  setKind: (k: string) => void;
}) {
  const { view, scope, kind, setKind } = p;
  if (!view.meta) return null;
  return (
    <>
      <p>
        Última leitura boa: {formatDate(view.meta.provenance.ended_at)} ·
        RouterOS {view.meta.provenance.version} · {view.meta.count} itens ·
        origem: concentrador via SSH.
      </p>
      {scope === 'catalog' && (
        <label>
          Filtrar por tipo
          <select
            aria-label="Filtrar por tipo"
            value={kind}
            onChange={(e) => {
              setKind(e.target.value);
              view.first();
            }}
          >
            <option value="">Todos</option>
            <option value="ether">Ethernet</option>
            <option value="bridge">Bridge</option>
            <option value="vlan">VLAN</option>
            <option value="service">Serviço PPPoE</option>
            <option value="wlan">Wireless</option>
          </select>
        </label>
      )}
      <p>
        {scope === 'catalog'
          ? `VLANs e faixas representam configuração. Não há vínculo individual de VLAN comprovado para as sessões. ` +
            (view.meta.absent_interfaces !== undefined
              ? `${view.meta.absent_interfaces} interfaces dinâmicas não vistas, ` +
                'retidas por até 24 h e separadas das presentes.'
              : '') +
            (view.meta.absences_limited
              ? ' Retenção de ausentes limitada para preservar a leitura atual.'
              : '')
          : 'Contadores acumulados desde o boot, agregados por servidor. Eles não identificam motivo de rejeição ' +
            'ou autenticação de um assinante. Motivo individual indisponível; nenhum log bruto é coletado.'}
      </p>
      {view.pageLoading && <p role="status">Carregando página armazenada…</p>}
      {!view.pageLoading && !view.pageError && view.rows.length === 0 && (
        <p>Nenhum item configurado nesta leitura completa.</p>
      )}
      {scope === 'radius' &&
        view.meta.count === 1 &&
        view.rows[0]?.kind === 'aaa' && (
          <p>Nenhum servidor RADIUS configurado nesta leitura.</p>
        )}
      {view.rows
        .filter((r) => !r.not_seen_since)
        .map((r) => (
          <DiagnosticRow key={r.id} row={r} />
        ))}
      {view.rows.some((r) => r.not_seen_since) && (
        <section aria-label="Interfaces dinâmicas não vistas na leitura atual">
          <h3>Não vistas na leitura atual</h3>
          {view.rows
            .filter((r) => r.not_seen_since)
            .map((r) => (
              <DiagnosticRow key={r.id} row={r} />
            ))}
        </section>
      )}
    </>
  );
}

export function PPPoEDiagnostics({
  profile,
  activation,
  device,
  scope,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  scope: 'catalog' | 'radius';
  blocked: boolean;
}) {
  const [kind, setKind] = useState('');
  const view = useDiagnostic(
    profile,
    activation,
    device,
    scope,
    undefined,
    kind,
  );
  const title =
    scope === 'catalog'
      ? 'Serviços, interfaces e VLANs'
      : 'RADIUS no concentrador';
  const actionLabel =
    view.starting || view.job
      ? 'Consultando…'
      : view.uncertain
        ? 'Consultar ação pendente'
        : scope === 'catalog'
          ? 'Atualizar serviços e VLANs'
          : 'Atualizar RADIUS';
  return (
    <section
      aria-label={title}
      className="pppoe-diagnostics"
      aria-busy={view.loading || view.pageLoading}
    >
      <h2>{title}</h2>
      <button
        className="primary"
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !!view.job ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => void view.start()}
      >
        {actionLabel}
      </button>
      {view.job && (
        <button onClick={() => void view.cancel()}>Cancelar consulta</button>
      )}
      {view.statusFailed && (
        <button onClick={view.retryStatus}>Consultar estado novamente</button>
      )}
      {view.error && <p role="alert">{view.error}</p>}
      {view.loading && <p role="status">Carregando diagnóstico armazenado…</p>}
      {!view.loading && !view.error && !view.meta && (
        <p>
          Nenhuma leitura deste diagnóstico disponível. Atualize para consultar
          o concentrador.
        </p>
      )}
      {!view.error &&
        view.attempt &&
        view.attempt.state !== 'RUNNING' &&
        view.attempt.code !== 'OK' && (
          <p role="status">
            Tentativa{' '}
            {view.attempt.quality?.status === 'limited'
              ? 'limitada'
              : 'incompleta'}
            : {diagnosticMessage(view.attempt.code)}{' '}
            {view.meta
              ? 'A última leitura boa e sua data foram preservadas.'
              : 'Nenhuma leitura boa disponível.'}
          </p>
        )}
      <DiagnosticListContent
        view={view}
        scope={scope}
        kind={kind}
        setKind={setKind}
      />
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {(view.error || view.pageError) && (
        <button onClick={view.reopen}>Reabrir dados armazenados</button>
      )}
      <div className="pppoe-actions">
        {view.cursor && (
          <button onClick={view.first}>Início do diagnóstico</button>
        )}
        {!view.pageLoading && !view.pageError && view.next && (
          <button onClick={view.more}>Próxima página do diagnóstico</button>
        )}
      </div>
    </section>
  );
}

function PingResultDetails(p: {
  result: NonNullable<PDRow['ping']>;
  metaDate: string;
}) {
  const { result, metaDate } = p;
  const baselineDiff =
    result.baseline_median_ns !== undefined &&
    result.baseline_median_ns > 0 &&
    result.avg_ns !== undefined
      ? ` · RTT médio ${result.avg_ns >= result.baseline_median_ns ? '+' : '−'}` +
        `${formatMetric((Math.abs(result.avg_ns - result.baseline_median_ns) * 100) / result.baseline_median_ns, '%')}`
      : '';
  const minText = formatLatency(result.min_ns);
  const avgText = formatLatency(result.avg_ns);
  const maxText = formatLatency(result.max_ns);
  const jitterText = formatLatency(result.jitter_ns);
  const baseMedText = formatLatency(result.baseline_median_ns);
  const samplesText =
    result.baseline_samples !== undefined
      ? `${result.baseline_samples}/10 amostras anteriores disponíveis.`
      : 'Histórico não registrado.';
  return (
    <>
      <p>
        Origem: {result.context} · {formatDate(metaDate)}
      </p>
      <p>
        Enviados: {result.sent} · Recebidos: {result.received} · Perda:{' '}
        {formatMetric(result.loss_percent, '%')}
      </p>
      <p>
        RTT mínimo:{' '}
        <span title={exactNumber(result.min_ns, 'ns')}>{minText}</span> · médio:{' '}
        <span title={exactNumber(result.avg_ns, 'ns')}>{avgText}</span> ·
        máximo: <span title={exactNumber(result.max_ns, 'ns')}>{maxText}</span>
      </p>
      <p>
        Amostra de 3 probes · jitter:{' '}
        <span title={exactNumber(result.jitter_ns, 'ns')}>{jitterText}</span>.
      </p>
      <p>
        Mediana dos 10 pings anteriores bem-sucedidos deste alvo e sessão:{' '}
        <span title={exactNumber(result.baseline_median_ns, 'ns')}>
          {baseMedText}
        </span>
        {baselineDiff}. {samplesText} Histórico em memória desde o início do
        serviço; perdas não entram na baseline.
      </p>
      <ol>
        {result.probes.map((probe) => (
          <li key={probe.sequence} value={probe.sequence + 1}>
            {probe.status || 'Resposta ICMP'}
            {probe.rtt_ns !== undefined
              ? ` · ${formatLatency(probe.rtt_ns)}`
              : ''}
          </li>
        ))}
      </ol>
    </>
  );
}

export function PPPoEPing({
  profile,
  activation,
  device,
  observation,
  row,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  blocked: boolean;
}) {
  const view = useDiagnostic(profile, activation, device, 'ping', {
    observation,
    row,
  });
  const result = view.rows[0]?.ping;
  const isTargetAttempt =
    view.attempt?.source_observation === observation &&
    view.attempt?.source_row === row;
  const pingLabel =
    view.starting || view.job
      ? 'Sondando…'
      : view.uncertain
        ? 'Consultar sondagem pendente'
        : 'Sondar IP pelo concentrador';
  return (
    <section
      aria-label="Sondagem da sessão pelo concentrador"
      className="pppoe-diagnostics"
    >
      <h3>Ping pelo concentrador</h3>
      <p>
        3 pacotes · intervalo de 200 ms · contexto main, quando comprovado. A
        sessão é revalidada antes da sondagem.
      </p>
      <button
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !!view.job ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => void view.start()}
      >
        {pingLabel}
      </button>
      {view.job && (
        <button onClick={() => void view.cancel()}>Cancelar sondagem</button>
      )}
      {view.statusFailed && (
        <button onClick={view.retryStatus}>Consultar estado da sondagem</button>
      )}
      {view.error && <p role="alert">{view.error}</p>}
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {view.loading && (
        <p role="status">Carregando última sondagem armazenada…</p>
      )}
      {!view.error &&
        isTargetAttempt &&
        view.attempt?.state !== 'RUNNING' &&
        view.attempt?.code !== 'OK' && (
          <p role="status">
            Tentativa de sondagem incompleta:{' '}
            {diagnosticMessage(view.attempt?.code || '')}{' '}
            {view.meta
              ? 'A última sondagem boa e sua data foram preservadas.'
              : 'Nenhuma sondagem boa disponível.'}
          </p>
        )}
      {!view.loading && !view.error && !view.meta && (
        <p>Nenhuma sondagem armazenada para esta sessão.</p>
      )}
      {(view.error || view.pageError) && (
        <button onClick={view.reopen}>Reabrir sondagem armazenada</button>
      )}
      {result && view.meta && (
        <PingResultDetails
          result={result}
          metaDate={view.meta.provenance.ended_at}
        />
      )}
      <p>
        Não responder não significa estar offline. A validação e a sondagem
        ocorrem em uma janela; resposta ICMP não comprova navegação saudável.
      </p>
    </section>
  );
}
