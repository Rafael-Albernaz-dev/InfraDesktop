import { useRadioLinkState } from './radio-link-state';
import { DataBlock } from './ui/DataBlock';
import type { RadioLinkPanelProps } from './radio-link-state';
import { RadioLinkContent } from './radio-link-content';
export function RadioLinkPanel(props: RadioLinkPanelProps & { session?: boolean }) {
  const { rootRef, ...state } = useRadioLinkState(props);
  const content = state.loading ? (
    <p role="status">Lendo vínculo armazenado…</p>
  ) : (
    <RadioLinkContent state={state} />
  );
  return (
    <section className="radio-panel" ref={rootRef} aria-label="Enlace rádio">
      {props.session ? (
        <DataBlock title="Enlace" source="SSH" data={{ state: 'ready', content }} />
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
