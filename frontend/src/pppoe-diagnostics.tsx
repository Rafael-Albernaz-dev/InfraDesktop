import { PPReadResultMetadata, PPReadResultView } from './pppoe-read-result';
import { RadioModes } from './radio-contract';
import { Button } from './ui/Button';
import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';

import type { PPReadPanelProps } from './pppoe-read-state';
import { usePPReadState } from './pppoe-read-state';
import { PPReadActionButtons, PPReadStatusBanners } from './pppoe-read-views';
export { DeviceMonitoring } from './device-monitoring';
export { PPPoEDiagnostics, PPPoEPing } from './pppoe-diag-views';

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
  if (props.actionsOnly)
    return (
      <>
        <PPReadActionButtons s={s} pProps={props} />
        <PPReadStatusBanners s={s} pProps={props} />
        {s.result && (
          <details className="det">
            <summary>Última leitura operacional</summary>
            <PPReadResultMetadata s={s} />
            <PPReadResultView s={s} pProps={{ ...props, summaryOnly: false }} />
          </details>
        )}
      </>
    );
  const heading = props.traffic
    ? 'Tráfego'
    : props.evidence === 'log'
      ? 'Log PPP'
      : props.evidence === 'secret'
        ? 'Evidência do secret'
        : props.monitoring
          ? 'Leitura operacional'
          : props.health
            ? props.healthView === 'uplinks'
              ? 'Interfaces estáticas'
              : props.healthView === 'pools'
                ? 'Pools IPv4'
                : 'Pools e interfaces estáticas'
            : props.bandwidthOnly
              ? 'Banda'
              : props.allowTop && !props.rows.length
                ? 'Top consumo'
                : 'Banda por sessão';
  const body = (
    <>
      {props.healthView !== 'pools' && (
        <div className="bar">
          {(props.pageBandwidth || props.healthView === 'uplinks') && (
            <>
              <span className="muted">
                {props.healthView === 'uplinks'
                  ? 'TX e RX vistos pela interface do concentrador.'
                  : 'Mede as sessões desta página.'}
              </span>
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
          {(props.pageBandwidth || props.healthView === 'uplinks') && (
            <span className="grow" />
          )}
          <PPReadActionButtons s={s} pProps={props} />
        </div>
      )}
      <PPReadStatusBanners s={s} pProps={props} />
      {s.result && <PPReadResultView s={s} pProps={props} />}
    </>
  );
  const panelBody = (
    <>
      {s.result ? (
        <PPReadResultView s={s} pProps={props} />
      ) : (
        <p>Ainda sem leitura</p>
      )}
      <PPReadStatusBanners s={s} pProps={props} />
      <div className="actions">
        <PPReadActionButtons s={s} pProps={props} />
      </div>
    </>
  );
  return (
    <section
      className="pppoe-diagnostics pppoe-read-panel"
      aria-label={
        props.health
          ? props.healthView === 'uplinks'
            ? 'Interfaces estáticas'
            : props.healthView === 'pools'
              ? 'Pools IPv4'
              : 'Pools e interfaces estáticas'
          : heading
      }
    >
      {(!props.panel || !props.traffic) && props.healthView !== 'pools' && (
        <div className="sp10" />
      )}
      <DataBlock
        title={heading}
        actions={
          props.healthView === 'pools' ? (
            <PPReadActionButtons s={s} pProps={props} />
          ) : undefined
        }
        source={props.panel ? `SSH (${props.device.name})` : 'SSH'}
        age={
          props.pageBandwidth
            ? '—'
            : readingAge(s.result?.ended_at) || (props.panel ? '—' : '')
        }
        data={{
          state: s.job && s.result && !props.traffic ? 'refreshing' : 'ready',
          content: props.panel ? panelBody : body,
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
    props.evidence,
  ].join('|');
  return <PPPoEReadContent key={key} {...props} />;
}
