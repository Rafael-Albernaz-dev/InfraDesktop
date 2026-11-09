import { Fragment } from 'react';
import {
  exactRate,
  exactVolume,
  formatMetric,
  formatRate,
  formatVolume,
  uptimeTitle,
} from './network-format';
import { formatPPUptime } from './pppoe-format';
import {
  cancelReadLabel,
  readingFlag,
  trafficReadNotice,
} from './pppoe-read-labels';
import type { PPReadPanelProps, ReadState } from './pppoe-read-state';
import type { PPReadResult, PPReadRow } from './presentation';
import type { PPRate } from './presentation-pppoe-types';
import { RadioModes } from './radio-contract';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';

export function RateValue({ rate }: { rate?: PPRate }) {
  return <span title={exactRate(rate)}>{formatRate(rate)}</span>;
}

export function PPSecretEvidence({ result }: { result: PPReadResult }) {
  return (
    <section aria-label="Evidência efêmera do secret">
      <p>
        Texto livre do administrador no roteador; pode conter dado pessoal e não
        é motivo verificado
      </p>
      <p>Evidência apenas nesta consulta; não é salva nem exportada.</p>
      {result.rows.length === 0 && <p>Nenhum secret local para este login.</p>}
      {result.rows.map((row, index) => {
        const secret = row.secret;
        const items = [
          ['Serviço', secret?.service],
          ['Perfil', secret?.profile],
          ['Última saída', secret?.last_logged_out],
          ['Último caller ID', secret?.last_caller_id],
          ['Último motivo de desconexão', secret?.last_disconnect_reason],
          ['Comentário (até 128 caracteres)', secret?.comment],
        ] as const;
        return (
          <section key={index} aria-label={`Secret ${index + 1}`}>
            <h4>Secret {index + 1}</h4>
            <dl className="pppoe-diagnostic-values pppoe-context-values">
              <dt>Desativado</dt>
              <dd>{readingFlag(secret?.disabled)}</dd>
              {items.map(([label, value]) => (
                <Fragment key={label}>
                  <dt>{label}</dt>
                  <dd>
                    {value === undefined
                      ? 'Não informado nesta leitura'
                      : value || 'Sem texto informado'}
                  </dd>
                </Fragment>
              ))}
            </dl>
          </section>
        );
      })}
    </section>
  );
}

type TriggerProps = { s: ReadState; pProps: PPReadPanelProps };
function BandwidthTriggers({ s, pProps }: TriggerProps) {
  return (
    pProps.rows.length > 0 &&
    !pProps.traffic && (
      <>
        <Button
          type="submit"
          disabled={s.locked}
          onClick={() => void s.start('bandwidth')}
        >
          {pProps.rows.length === 1 ? 'Medir banda' : 'Medir banda da página'}
        </Button>
        {!pProps.bandwidthOnly && !pProps.pageBandwidth && (
          <Button
            type="submit"
            disabled={s.locked}
            onClick={() => void s.start(RadioModes.live)}
          >
            Ver ao vivo
          </Button>
        )}
      </>
    )
  );
}
function EvidenceTriggers({ s, pProps }: TriggerProps) {
  return (
    pProps.logRow && (
      <>
        {pProps.evidence !== 'secret' && (
          <Button
            type="submit"
            disabled={s.locked}
            onClick={() => void s.start('log')}
          >
            Log PPP
          </Button>
        )}
        {pProps.evidence !== 'log' && (
          <Button
            type="submit"
            disabled={s.locked}
            onClick={() => void s.start('secret')}
          >
            Ver evidência do secret
          </Button>
        )}
      </>
    )
  );
}
function PPReadTriggerButtons(p: {
  s: ReadState & { start: (mode: string) => Promise<void>; locked: boolean };
  pProps: PPReadPanelProps;
}) {
  const { s, pProps } = p;
  return (
    <>
      {pProps.monitoring && (
        <Button
          type="submit"
          className={pProps.summaryOnly ? undefined : 'primary'}
          disabled={s.locked}
          onClick={() => void s.start('operations')}
        >
          Atualizar monitoramento
        </Button>
      )}
      {pProps.health && (
        <Button
          type="submit"
          disabled={s.locked}
          onClick={() => void s.start('health')}
        >
          Atualizar
        </Button>
      )}
      <BandwidthTriggers s={s} pProps={pProps} />
      {pProps.traffic && !s.locked && (s.stoppedTraffic || s.error) && (
        <Button type="submit" onClick={() => void s.start('traffic')}>
          Ver ao vivo
        </Button>
      )}
      {pProps.allowTop && (
        <Button
          type="submit"
          disabled={s.locked}
          onClick={() => void s.start('top')}
        >
          Top consumo
        </Button>
      )}
      <EvidenceTriggers s={s} pProps={pProps} />
    </>
  );
}

export function PPReadActionButtons(p: {
  s: ReadState & {
    start: (mode: string) => Promise<void>;
    cancel: () => Promise<void>;
    locked: boolean;
  };
  pProps: PPReadPanelProps;
}) {
  const { s, pProps } = p;
  const cancelText = cancelReadLabel(s, pProps);
  return (
    <>
      <PPReadTriggerButtons s={s} pProps={pProps} />
      {(s.job || s.pending || s.starting || s.renewing) && (
        <Button
          type="submit"
          disabled={s.cancelling}
          onClick={() => void s.cancel()}
        >
          {cancelText}
        </Button>
      )}
      {(s.error || s.deadlineReached) && s.job && (
        <Button type="submit" onClick={() => s.retryPoll((n) => n + 1)}>
          Conferir andamento
        </Button>
      )}
    </>
  );
}

function progressStatusText(scope: string): string {
  if (scope === 'traffic') return 'Aguardando primeira amostra.';
  if (scope === RadioModes.live) {
    return 'Observação limitada a 60 s; atualização aproximada a cada 2 s.';
  }
  if (scope === 'operations')
    return 'Consultando projeções operacionais somente leitura…';
  if (scope === 'log') return 'Consultando eventos PPP deste login…';
  if (scope === 'secret') return 'Consultando evidência efêmera do secret…';
  return 'Aguardando duas amostras do concentrador…';
}

export function PPReadStatusBanners(p: {
  s: ReadState;
  pProps: PPReadPanelProps;
}) {
  const { s, pProps } = p;
  const trafficNotice = trafficReadNotice(s);
  return (
    <>
      <ReadProgress s={s} pProps={pProps} />
      {pProps.traffic && (
        <p className="muted">
          {trafficNotice} Gráfico retido em memória por 5 min.
        </p>
      )}
      {s.stoppedTraffic && <p role="status">Observação parada.</p>}
      {s.error && <p role="alert">{s.error}</p>}
      {pProps.monitoring && s.firstStatus !== undefined && (
        <p>
          Estado do job consultável após {formatMetric(s.firstStatus, 'ms')} ·
          consulta de estado (UI → serviço):{' '}
          {formatMetric(s.statusLatency, 'ms')}.
        </p>
      )}
      {s.error && s.failureReason && (
        <details>
          <summary>Detalhe técnico</summary>
          <p>Motivo: {s.failureReason}</p>
        </details>
      )}
    </>
  );
}

const PPP_LOG_LABELS: Record<string, string> = {
  auth_failed: 'Falha de autenticação',
  address_failed: 'Endereço IP indisponível',
  radius_timeout: 'Servidor RADIUS sem resposta',
  timeout: 'Tempo esgotado',
  disconnected: 'Conexão encerrada',
  connected: 'Conexão estabelecida',
  other: 'Evento PPP sem classificação',
};

export function PPReadLogTable({ rows }: { rows: PPReadResult['rows'] }) {
  return (
    <div className="table-region" tabIndex={0} aria-label="Eventos PPP">
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Horário no equipamento</th>
            <th>Evento</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>
              <td>{row.at || 'Horário indisponível'}</td>
              <td>
                {PPP_LOG_LABELS[row.event || 'other'] ||
                  'Evento PPP sem classificação'}
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}

type BandwidthRowProps = {
  r: PPReadRow;
  calculated: boolean;
  sampleAge: number;
};
function BandwidthRow({ r, calculated, sampleAge }: BandwidthRowProps) {
  const calcUptime =
    r.uptime.known && calculated
      ? {
          ...r.uptime,
          min: r.uptime.min + sampleAge * 1e6,
          max: r.uptime.max + sampleAge * 1e6,
        }
      : r.uptime;
  return (
    <tr>
      <td>
        {r.login || 'Não informado nesta leitura'}
        {r.name && <small className="cell-sub">{r.name}</small>}
      </td>
      <td>
        <RateValue rate={r.download} />
      </td>
      <td>
        <RateValue rate={r.upload} />
      </td>
      <td>
        <span title={uptimeTitle(r.uptime)}>
          {formatPPUptime(calcUptime, true)}
        </span>
        {calculated ? ' (calculado)' : ''}
      </td>
    </tr>
  );
}

type PPReadBandwidthTableProps = {
  rows: PPReadResult['rows'];
  calculated: boolean;
  sampleAge: number;
};
export function PPReadBandwidthTable(p: PPReadBandwidthTableProps) {
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Tabela de banda e consumo"
    >
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Usuário</th>
            <th title="TX concentrador">Download do assinante</th>
            <th title="RX concentrador">Upload do assinante</th>
            <th>Conectado há</th>
          </tr>
        </thead>
        <tbody>
          {p.rows.map((r, i) => (
            <BandwidthRow
              key={r.row_id || i}
              r={r}
              calculated={p.calculated}
              sampleAge={p.sampleAge}
            />
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}

export function PPReadCostInfo({
  cost,
  scope,
}: {
  cost: NonNullable<PPReadResult['cost']>;
  scope: string;
}) {
  const tunnelsLabel = scope === 'health' ? 'interfaces estáticas' : 'túneis';
  const cpuText =
    cost.cpu_start !== undefined && cost.cpu_end !== undefined
      ? ` · CPU observada: ${formatMetric(cost.cpu_start, '%')} / ${formatMetric(cost.cpu_end, '%')}`
      : '';
  return (
    <p>
      {cost.ssh_commands} comandos SSH · {cost.first_tunnels}/
      {cost.last_tunnels} {tunnelsLabel} nas amostras ·{' '}
      <span title={exactVolume(cost.ssh_bytes)}>
        {formatVolume(cost.ssh_bytes)}
      </span>
      {cpuText}
    </p>
  );
}

function ReadProgress({ s, pProps }: TriggerProps) {
  return s.deadlineReached ? (
    <p role="status">
      Prazo da observação encerrado. Consulte o estado para confirmar o término.
    </p>
  ) : (
    (s.starting || s.job) && !s.error && !pProps.traffic && (
      <p role="status">{progressStatusText(s.scope)}</p>
    )
  );
}
