import { useRadioLinkState } from './radio-link-state';
import type { RadioLinkPanelProps, RadioLinkState } from './radio-link-state';
import {
  RadioLinkCredentialErrors,
  RadioStoredLink,
} from './radio-link-summary';
import {
  RadioIndexRefresh,
  RadioIndexCredentialErrors,
  RadioIndexPagination,
  RadioLinkMatchReason,
} from './radio-link-index';
import {
  RadioLinkProposal,
  RadioLinkEditActions,
  RadioLegacySuggestions,
  RadioManualLinkForm,
} from './radio-link-association';
import {
  RadioLinkRunActions,
  RadioLinkCost,
  RadioLastMeasurement,
  RadioLinkBusyStatus,
  RadioTickSeries,
  RadioLinkReadings,
} from './radio-link-details';

function RadioLinkContent({ state }: { state: RadioLinkState }) {
  return (
    <>
      <RadioLinkCredentialErrors state={state} />
      <RadioStoredLink state={state} />
      <RadioIndexRefresh state={state} />
      <RadioIndexCredentialErrors state={state} />
      <RadioIndexPagination state={state} />
      <RadioLinkMatchReason state={state} />
      <RadioLinkProposal state={state} />
      <RadioLinkEditActions state={state} />
      <RadioLegacySuggestions state={state} />
      <RadioManualLinkForm state={state} />
      <RadioLinkRunActions state={state} />
      <RadioLinkCost state={state} />
      <RadioLastMeasurement state={state} />
      <RadioLinkBusyStatus state={state} />
      <RadioTickSeries state={state} />
      <RadioLinkReadings state={state} />
    </>
  );
}
export function RadioLinkPanel(props: RadioLinkPanelProps) {
  const { rootRef, ...state } = useRadioLinkState(props);
  return (
    <section className="radio-panel" ref={rootRef} aria-label="Enlace rádio">
      <h3>Enlace rádio</h3>
      <p>Uptime PPPoE: {state.pppoeUptime}</p>
      {state.loading ? (
        <p role="status">Lendo vínculo armazenado…</p>
      ) : (
        <RadioLinkContent state={state} />
      )}
    </section>
  );
}
