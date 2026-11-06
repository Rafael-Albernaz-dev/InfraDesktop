import { PPReadResultMetadata, PPReadResultView } from './pppoe-read-result';
import { RadioModes } from './radio-contract';
import { Button } from './ui/Button';

import type { PPReadPanelProps, ReadState } from './pppoe-read-state';
import { PPReadActionButtons, PPReadStatusBanners } from './pppoe-read-views';

type ReadViewProps = { s: ReadState; props: PPReadPanelProps };
export function ReadActionsOnly({ s, props }: ReadViewProps) {
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
}
function ReadStandardBar({ s, props }: ReadViewProps) {
  return (
    props.healthView !== 'pools' && (
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
    )
  );
}
export function ReadStandardBody({ s, props }: ReadViewProps) {
  return (
    <>
      <ReadStandardBar s={s} props={props} />
      <PPReadStatusBanners s={s} pProps={props} />
      {s.result && <PPReadResultView s={s} pProps={props} />}
    </>
  );
}
export function ReadPanelBody({ s, props }: ReadViewProps) {
  return (
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
      {props.evidence === 'log' && (
        <p className="muted">
          Até 50 eventos, com horário informado pelo equipamento.
        </p>
      )}
      {props.evidence === 'secret' && (
        <p className="muted">
          Apenas nesta consulta. Não é salva nem exportada, e a senha nunca é
          exibida.
        </p>
      )}
    </>
  );
}
