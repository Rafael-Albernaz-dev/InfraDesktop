import type { RadioCost } from './radio-api';
import type { RadioLinkState } from './radio-link-state';
import { RadioModes } from './radio-contract';
import { formatDate, formatMetric } from './network-format';
import { Reading } from './radio-reading';
export function RadioLinkRunActions({ state }: { state: RadioLinkState }) {
  const { blocked, radio, editing, link, cost } = state;
  return (
    <>
      {radio.busy ? (
        <button className="primary" onClick={() => void radio.stop()}>
          Parar
        </button>
      ) : (
        link &&
        !editing && (
          <div className="radio-actions">
            <button
              className="primary"
              disabled={blocked}
              onClick={() => void radio.start(RadioModes.once)}
            >
              Consultar enlace
            </button>
            <button
              disabled={blocked}
              onClick={() => void radio.start(RadioModes.measure)}
            >
              Medir custo do live
            </button>
            <button
              disabled={blocked || !cost?.enabled}
              onClick={() => void radio.start(RadioModes.live)}
            >
              Live curto
            </button>
          </div>
        )
      )}
    </>
  );
}
export function RadioLinkCost({ state }: { state: RadioLinkState }) {
  const { editing, link, cost } = state;
  return (
    <>
      {link && !editing && (
        <p>
          {cost
            ? `${cost.reason} Intervalo: ${formatMetric(cost.interval_seconds, 's')} · p95 ${formatMetric(cost.p95_seconds, 's')}`
            : 'Live desligado: custo dos ticks ainda não medido nesta execução.'}
        </p>
      )}
    </>
  );
}
export function RadioLastMeasurement({ state }: { state: RadioLinkState }) {
  const { editing, link, cost } = state;
  return (
    <>
      {link && !editing && cost?.measurements?.length === 10 && (
        <p>
          Última amostra — AP: comando{' '}
          {formatMetric(cost.measurements[9].ap.command_seconds, 's')}, percurso{' '}
          {formatMetric(cost.measurements[9].ap.path_seconds, 's')}
          {!cost.ap_only && (
            <>
              ; CPE: comando{' '}
              {formatMetric(cost.measurements[9].cpe.command_seconds, 's')},
              percurso{' '}
              {formatMetric(cost.measurements[9].cpe.path_seconds, 's')}
            </>
          )}
          .
        </p>
      )}
    </>
  );
}
export function RadioLinkBusyStatus({ state }: { state: RadioLinkState }) {
  const { radio } = state;
  return (
    <>
      {radio.busy && (
        <p role="status">
          Consulta em curso; um enlace por vez. A janela do live termina em até
          90 s.
        </p>
      )}
    </>
  );
}
export function RadioTickSeries({ state }: { state: RadioLinkState }) {
  const { cost } = state;
  return (
    <>
      {!!cost?.series?.length && (
        <details>
          <summary>Série por tick ({cost.series.length})</summary>
          <ol aria-label="Recibo dos ticks">
            {cost.series.map((tick, i) => (
              <RadioTickRow
                key={tick.observed_at + '|' + i}
                tick={tick}
                apOnly={cost.ap_only}
              />
            ))}
          </ol>
        </details>
      )}
    </>
  );
}
export function RadioLinkReadings({ state }: { state: RadioLinkState }) {
  const { radio, link } = state;
  const hasReading = !!(radio.view?.reading || radio.view?.cpe_reading);
  return (
    <>
      {hasReading && (
        <div className="radio-sides">
          <Reading
            title="Lado AP"
            reading={radio.view?.reading}
            code={radio.view?.ap_code}
            onlyMAC={link?.mac}
            note={false}
          />
          {radio.view?.cpe_note ? (
            <p className="radio-note">{radio.view.cpe_note}</p>
          ) : (
            <Reading
              title="Lado CPE"
              reading={radio.view?.cpe_reading}
              code={radio.view?.cpe_code}
              note={false}
            />
          )}
        </div>
      )}
      {hasReading && (
        <p className="radio-note">
          Taxa negociada, não é velocidade. Dados ausentes permanecem
          indisponíveis.
        </p>
      )}
    </>
  );
}

function RadioTickRow({
  tick,
  apOnly,
}: {
  tick: NonNullable<RadioCost['series']>[number];
  apOnly: boolean | undefined;
}) {
  return (
    <li>
      <p>
        {formatDate(tick.observed_at)} · CPU{' '}
        {(tick.cpu ?? []).length
          ? (tick.cpu ?? []).map((v) => formatMetric(v, '%')).join(' / ')
          : 'indisponível'}{' '}
        · comando AP{' '}
        {tick.ap_command_seconds > 0
          ? formatMetric(tick.ap_command_seconds, 's')
          : 'indisponível'}{' '}
        {!apOnly && (
          <>
            {' '}
            / CPE{' '}
            {tick.cpe_command_seconds > 0
              ? formatMetric(tick.cpe_command_seconds, 's')
              : 'indisponível'}
          </>
        )}{' '}
        · ICMP{' '}
        {(tick.icmp_rtt_ms ?? []).length
          ? (tick.icmp_rtt_ms ?? [])
              .map((v) => formatMetric(v, 'ms'))
              .join(' / ')
          : 'indisponível'}{' '}
        · {tick.reason}
      </p>
    </li>
  );
}
