import { usePPPoEReadJob } from './use-pppoe-read-job';
import { useTrafficRenewal } from './use-traffic-renewal';
import { RadioModes } from '../radio/radio-contract';
import type { PPReadPanelProps } from './pppoe-read-props';
export type { PPReadPanelProps } from './pppoe-read-props';
export type ReadState = Omit<ReturnType<typeof usePPReadState>, 'setRoot'>;
export function usePPReadState(props: PPReadPanelProps) {
  const { state: s, context, actions, doc, setRoot } = usePPPoEReadJob(props);
  useTrafficRenewal(context, doc, Boolean(props.traffic));
  const locked = Boolean(
    props.blocked ||
    s.starting ||
    s.pending ||
    s.renewing ||
    s.job ||
    s.cancelling ||
    s.pageBusy,
  );
  const calculated = Boolean(
    s.job &&
    (s.scope === RadioModes.live || s.scope === 'traffic') &&
    !s.error &&
    !s.deadlineReached,
  );
  return { ...s, ...actions, setRoot, locked, calculated };
}
