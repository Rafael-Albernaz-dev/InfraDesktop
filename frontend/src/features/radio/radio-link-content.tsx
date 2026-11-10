import type { RadioLinkState } from './radio-link-state';
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

export function RadioLinkContent({
  state,
  session = false,
}: {
  state: RadioLinkState;
  session?: boolean;
}) {
  return (
    <>
      <RadioLinkCredentialErrors state={state} />
      {!session && <RadioStoredLink state={state} />}
      {session && state.link && !state.link.confirmed_now && (
        <p role="status">
          Vínculo não confirmado agora. O cadastro foi preservado.
        </p>
      )}
      <RadioIndexRefresh state={state} />
      <RadioIndexCredentialErrors state={state} />
      <RadioIndexPagination state={state} />
      <RadioLinkMatchReason state={state} />
      <RadioLinkProposal state={state} />
      {session ? (
        <div className="actions">
          <RadioLinkRunActions state={state} part="update" />
          <RadioLinkEditActions state={state} />
        </div>
      ) : (
        <RadioLinkEditActions state={state} />
      )}
      <RadioLegacySuggestions state={state} />
      <RadioManualLinkForm state={state} />
      <RadioLinkRunActions
        state={state}
        part={session ? 'measurement' : 'all'}
      />
      <RadioLinkCost state={state} session={session} />
      <RadioLastMeasurement state={state} />
      <RadioLinkBusyStatus state={state} />
      <RadioTickSeries state={state} />
      {(!session ||
        state.radio.view?.reading ||
        state.radio.view?.cpe_reading) && <RadioLinkReadings state={state} />}
    </>
  );
}
