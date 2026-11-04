import { RadioModes } from './radio-contract';
import { Button } from './ui/Button';
import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';
import { PPReadScopeBody, PPReadResultPagination } from './pppoe-read-result';

import {
  exactNumber,
  formatDate,
  formatDuration,
  formatLatency,
} from './network-format';
import type { PPReadPanelProps } from './pppoe-read-state';
import { usePPReadState } from './pppoe-read-state';
import {
  PPReadActionButtons,
  PPReadCostInfo,
  PPReadStatusBanners,
} from './pppoe-read-views';
export { DeviceMonitoring } from './device-monitoring';
export { PPPoEDiagnostics, PPPoEPing } from './pppoe-diag-views';
type ReadState = ReturnType<typeof usePPReadState>;

function PPReadResultMetadata({ s }: { s: ReadState }) {
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
function PPReadResultView({
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
function PPPoEReadContent(props: PPReadPanelProps) {
  const s = usePPReadState(props);
  if (
    !props.rows.length &&
    !props.allowTop &&
    !props.logRow &&
    !props.health &&
    !props.monitoring
  ) {
    return null;
  }
  const heading = props.traffic
    ? 'Tráfego'
    : props.monitoring
      ? 'Leitura operacional'
      : props.health
        ? 'Pools e interfaces estáticas'
        : 'Banda por sessão';
  const body = (
    <>
      <div className="bar">
        {props.pageBandwidth && (
          <>
            <span className="muted">Mede as sessões desta página.</span>
          </>
        )}
        {(s.result || props.pageBandwidth) && (
          <details className="det">
            <summary>Detalhes da leitura</summary>
            {s.result && <PPReadResultMetadata s={s} />}
            {props.pageBandwidth && (
              <Button
                disabled={s.locked}
                onClick={() => void s.start(RadioModes.live)}
              >
                Ver ao vivo
              </Button>
            )}
          </details>
        )}
        {props.pageBandwidth && <span className="grow" />}
        <PPReadActionButtons s={s} pProps={props} />
      </div>
      <PPReadStatusBanners s={s} pProps={props} />
      {s.result && <PPReadResultView s={s} pProps={props} />}
    </>
  );
  return (
    <section
      className="pppoe-diagnostics pppoe-read-panel"
      aria-label={heading}
    >
      <div className="sp10" />
      <DataBlock
        title={heading}
        source="SSH"
        age={props.pageBandwidth ? '—' : readingAge(s.result?.ended_at)}
        data={{
          state: s.job && s.result ? 'refreshing' : 'ready',
          content: body,
        }}
      />
    </section>
  );
}
export function PPPoEReadPanel(props: PPReadPanelProps) {
  const key = [
    props.profile,
    props.activation,
    props.device.id,
    props.device.revision,
    props.observation,
    props.rows.join(','),
    props.allowTop,
    props.logRow,
    props.health,
    props.monitoring,
    props.traffic,
  ].join('|');
  return <PPPoEReadContent key={key} {...props} />;
}
