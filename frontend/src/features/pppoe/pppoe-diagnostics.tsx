import { readingAge } from '../equipment/equipment-reading-format';
import { readHeading, hasReadTarget } from './pppoe-read-heading';
import {
  ReadActionsOnly,
  ReadPanelBody,
  ReadStandardBody,
} from './pppoe-read-panel-body';
import { DataBlock } from '../../shared/ui/DataBlock';

import type { PPReadPanelProps, ReadState } from './pppoe-read-state';
import { usePPReadState } from './pppoe-read-state';
import { PPReadActionButtons } from './pppoe-read-views';
export { PPPoEDiagnostics } from './pppoe-diag-views';
export { PPPoEPing } from './pppoe-ping';

function readAge(props: PPReadPanelProps, s: ReadState) {
  if (props.pageBandwidth) return '—';
  return readingAge(s.result?.ended_at) || (props.panel ? '—' : '');
}
function PPPoEReadContent(props: PPReadPanelProps) {
  const { setRoot, ...s } = usePPReadState(props);
  if (!hasReadTarget(props)) return null;
  if (props.actionsOnly) return <ReadActionsOnly s={s} props={props} />;
  const heading = readHeading(props);
  return (
    <section
      ref={setRoot}
      className="pppoe-diagnostics pppoe-read-panel"
      aria-label={heading}
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
        age={readAge(props, s)}
        data={{
          state: s.job && s.result && !props.traffic ? 'refreshing' : 'ready',
          content: props.panel ? (
            <ReadPanelBody s={s} props={props} />
          ) : (
            <ReadStandardBody s={s} props={props} />
          ),
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
