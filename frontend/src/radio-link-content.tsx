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

export function RadioLinkContent({ state }: { state: RadioLinkState }) {
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
