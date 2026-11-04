import { Button } from './ui/Button';
import { SessionTraffic } from './session-traffic';
import { DeviceOperationalTables } from './device-monitoring';

import type { PPReadPanelProps } from './pppoe-read-state';
import { usePPReadState } from './pppoe-read-state';
import {
  PPHealthTables,
  PPReadBandwidthTable,
  PPReadLogTable,
  PPSecretEvidence,
} from './pppoe-read-views';
type ReadState = ReturnType<typeof usePPReadState>;
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
            live={Boolean(s.job && !s.stoppedTraffic && !s.deadlineReached)}
          />
        ))}
      </>
    );
  }
  if (result.scope === 'secret') return <PPSecretEvidence result={result} />;
  if (result.scope === 'health') return <PPHealthTables result={result} />;
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
