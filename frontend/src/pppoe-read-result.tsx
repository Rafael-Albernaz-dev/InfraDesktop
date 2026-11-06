import {
  exactNumber,
  formatDate,
  formatDuration,
  formatLatency,
} from './network-format';
import { Button } from './ui/Button';
import { SessionTraffic } from './session-traffic';
import { DeviceOperationalTables } from './device-monitoring';

import type { PPReadPanelProps,ReadState } from './pppoe-read-state';
import {
  PPReadCostInfo,
  PPHealthTables,
  PPReadBandwidthTable,
  PPReadLogTable,
  PPSecretEvidence,
} from './pppoe-read-views';
export function PPReadScopeBody(p: { s: ReadState; pProps: PPReadPanelProps }) {
  const { s, pProps } = p;
  const result = s.result!;
  if (result.scope === 'operations') {
    return pProps.summaryOnly ? null : (
      <DeviceOperationalTables result={result} />
    );
  }
  if (result.scope === 'traffic') {
    return (
      <>
        {result.rows.map((row) => (
          <SessionTraffic
            key={row.row_id}
            row={row}
            startedAt={result.started_at}
            live={Boolean(s.job && !s.stoppedTraffic && !s.deadlineReached)}
          />
        ))}
      </>
    );
  }
  if (result.scope === 'secret') return <PPSecretEvidence result={result} />;
  if (result.scope === 'health')
    return <PPHealthTables result={result} view={pProps.healthView} />;
  if (result.rows.length === 0) {
    const emptyMsg =
      result.scope === 'operations'
        ? 'Consultando projeções operacionais somente leitura…'
        : s.scope === 'log'
          ? 'Nenhum evento PPP reconhecido para este login.'
          : 'Nenhum túnel ativo na leitura concluída.';
    return <p>{emptyMsg}</p>;
  }
  if (result.scope === 'log') return <PPReadLogTable rows={result.rows} />;
  return (
    <PPReadBandwidthTable
      rows={result.rows}
      calculated={s.calculated}
      sampleAge={s.sampleAge}
    />
  );
}
export function PPReadResultPagination(p: {
  s: ReadState;
  pProps: PPReadPanelProps;
}) {
  const { s, pProps } = p;
  const result = s.result!;
  const nextLabel =
    result.scope === 'operations'
      ? 'Próxima página do monitoramento'
      : result.scope === 'health'
        ? 'Próxima página de pools e interfaces'
        : result.scope === 'log'
          ? 'Próximos eventos PPP'
          : 'Próxima página de consumo';
  const firstLabel =
    result.scope === 'health'
      ? 'Início de pools e interfaces'
      : result.scope === 'log'
        ? 'Primeiros eventos PPP'
        : 'Início do consumo';
  const isScoped =
    result.scope === 'top' ||
    result.scope === 'operations' ||
    result.scope === 'log' ||
    result.scope === 'health';
  const hasFirst = Boolean(s.resultJob && isScoped && s.pageCursor);
  return (
    <>
      {s.next && !pProps.summaryOnly && (
        <Button
          type="submit"
          disabled={s.pageBusy}
          onClick={() => void s.page(s.next)}
        >
          {nextLabel}
        </Button>
      )}
      {hasFirst && (
        <Button
          type="submit"
          disabled={s.pageBusy}
          onClick={() => void s.page('')}
        >
          {firstLabel}
        </Button>
      )}
    </>
  );
}

export function PPReadResultMetadata({ s }: { s: ReadState }) {
  const result = s.result!;
  return (
    <>
      <p>
        {s.error || s.deadlineReached ? 'Último resultado de ' : 'Leitura de '}
        {formatDate(result.ended_at)} · duração{' '}
        <span title={exactNumber(result.duration_ns / 1e9, 's')}>
          {formatDuration(result.duration_ns / 1e9)}
        </span>
      </p>
    </>
  );
}

export function PPReadResultView({
  s,
  pProps,
}: {
  s: ReadState;
  pProps: PPReadPanelProps;
}) {
  const result = s.result!;
  return (
    <>
      {result.scope === 'operations' && (
        <dl className="pppoe-diagnostic-values">
          <dt>Handshake SSH</dt>
          <dd title={exactNumber(result.cost?.ssh_handshake_ns, 'ns')}>
            {formatLatency(result.cost?.ssh_handshake_ns)}
          </dd>
          <dt>Coletores</dt>
          <dd title={exactNumber(result.cost?.collector_ns, 'ns')}>
            {formatLatency(result.cost?.collector_ns)}
          </dd>
        </dl>
      )}
      {result.scope === 'top' && (
        <p>
          Consumo ordenado pelo limite inferior de download + upload. Valores
          indisponíveis aparecem ao final.
        </p>
      )}
      {result.scope !== 'log' &&
        result.scope !== 'secret' &&
        result.scope !== 'operations' &&
        result.cost && (
          <PPReadCostInfo cost={result.cost} scope={result.scope} />
        )}
      {result.scope === 'log' && (
        <p>
          Até 50 eventos PPP, sem depuração. Horários informados pelo
          equipamento; categorias não comprovam a causa da falha.
        </p>
      )}
      <PPReadScopeBody s={s} pProps={pProps} />
      <PPReadResultPagination s={s} pProps={pProps} />
    </>
  );
}
