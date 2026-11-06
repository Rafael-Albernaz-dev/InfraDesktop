import { useRadioLinkState } from './radio-link-state';
import { DataBlock } from './ui/DataBlock';
import type { RadioLinkPanelProps } from './radio-link-state';
import { RadioLinkContent } from './radio-link-content';
import { useSessionLinkDetails } from './session-link-details';
import { SessionLinkSummary } from './session-link-summary';
import { readingAge } from './equipment-reading-format';
export function RadioLinkPanel(
  props: RadioLinkPanelProps & { session?: boolean },
) {
  const { rootRef, ...state } = useRadioLinkState(props);
  const details = useSessionLinkDetails(state, Boolean(props.session));
  const content = state.loading ? (
    <p role="status">Lendo vínculo armazenado…</p>
  ) : (
    <>
      {props.session && state.link && (
        <SessionLinkSummary
          link={state.link}
          names={details.names}
          reading={details.reading}
          open={state.onOpenEquipment}
        />
      )}
      <RadioLinkContent state={state} session={props.session} />
    </>
  );
  return (
    <section className="radio-panel" ref={rootRef} aria-label="Enlace rádio">
      {props.session ? (
        <DataBlock
          title="Enlace"
          source="SSH"
          age={readingAge(details.reading?.observed_at, state.now)}
          data={{ state: 'ready', content }}
        />
      ) : (
        <>
          <h3>Enlace rádio</h3>
          <p>Uptime PPPoE: {state.pppoeUptime}</p>
          {content}
        </>
      )}
    </section>
  );
}
