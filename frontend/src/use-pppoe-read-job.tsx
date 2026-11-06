import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from 'react';
import { initialReadState, readReducer } from './pppoe-read-machine';
import type { ReadEvent } from './pppoe-read-machine';
import { createReadLifecycle } from './pppoe-read-lifecycle';
import type { ReadContext } from './pppoe-read-lifecycle';
import type { PPReadPanelProps } from './pppoe-read-props';
import { startRead } from './pppoe-read-start';
import { cancelRead, pageRead } from './pppoe-read-cancel';
import { pollRead } from './pppoe-read-poll';
import { scheduleTraffic, trafficFailed } from './use-traffic-renewal';
import { pppoeReadAPI as api } from './presentation';
import { RadioModes } from './radio-contract';
function useReadClock(c: ReadContext, job: string, scope: string) {
  useEffect(() => {
    if (scope !== RadioModes.live && scope !== 'traffic') return;
    if (!job && scope === RadioModes.live) return;
    const tick = () => {
      const now = performance.now(),
        values = { sampleAge: Math.max(0, now - c.life.current.sampleClock) };
      c.send({ type: 'metadata', values });
      if (
        scope === RadioModes.live &&
        job &&
        now - c.life.current.liveStarted >= 60000
      ) {
        c.send({ type: 'metadata', values: { deadlineReached: true } });
        clearInterval(timer);
      }
    };
    const timer = setInterval(tick, 1000),
      initial = setTimeout(tick, 0);
    return () => {
      clearInterval(timer);
      clearTimeout(initial);
    };
  }, [c, job, scope]);
}
function readLifetime(
  c: ReadContext,
  profile: string,
  activation: string,
  deviceId: string,
) {
  const l = c.life.current;
  l.disposed = false;
  return () => {
    l.disposed = true;
    clearTimeout(l.renewalTimer);
    clearTimeout(l.pollTimer);
    l.generation++;
    const id = l.activeJob || l.privateJob,
      op = l.operation;
    l.activeJob = '';
    l.privateJob = '';
    l.operation = '';
    if (id) void api.cancel(profile, activation, id).catch(() => {});
    else if (op)
      void api
        .cancelOperation(profile, activation, deviceId, op)
        .catch(() => {});
  };
}
function useReadCleanup(c: ReadContext, p: PPReadPanelProps) {
  const {
    profile,
    activation,
    device: { id: deviceId },
  } = p;
  useEffect(
    () => readLifetime(c, profile, activation, deviceId),
    [c, profile, activation, deviceId],
  );
}
function automaticTraffic(c: ReadContext) {
  const p = c.latest.current,
    l = c.life.current;
  if (!p.traffic || p.blocked || !p.rows.length || l.automaticStarted) return;
  l.automaticStarted = true;
  void c.start('traffic');
}
function useAutomaticTraffic(c: ReadContext, p: PPReadPanelProps) {
  const {
    traffic,
    blocked,
    rows: { length: rowCount },
  } = p;
  useEffect(() => automaticTraffic(c), [c, traffic, blocked, rowCount]);
}
export function usePPPoEReadJob(p: PPReadPanelProps) {
  const [state, dispatch] = useReducer(readReducer, initialReadState);
  const snapshot = useRef(state),
    latest = useRef(p),
    life = useRef(createReadLifecycle());
  useLayoutEffect(() => {
    latest.current = p;
  }, [p]);
  const send = useCallback((e: ReadEvent) => {
    snapshot.current = readReducer(snapshot.current, e);
    dispatch(e);
  }, []);
  const context = useMemo(() => {
    const c: ReadContext = {
      life,
      latest,
      state: snapshot,
      send,
      start: (mode, preserve) => startRead(c, mode, preserve),
      schedule: (delay) => scheduleTraffic(c, delay),
      failed: (code) => trafficFailed(c, code),
    };
    return c;
  }, [send]);
  useReadCleanup(context, p);
  useAutomaticTraffic(context, p);
  useEffect(() => {
    if (!state.job) return;
    void pollRead(context, context.life.current.generation);
    return () => clearTimeout(context.life.current.pollTimer);
  }, [context, state.job, state.pollRetry]);
  useReadClock(context, state.job, state.scope);
  const actions = useMemo(
    () => ({
      start: (mode: string, preserve?: boolean) =>
        context.start(mode, preserve),
      cancel: () => cancelRead(context),
      page: (cursor: string) => pageRead(context, cursor),
      retryPoll: (value: React.SetStateAction<number>) =>
        send({
          type: 'metadata',
          values: {
            pollRetry:
              typeof value === 'function'
                ? value(snapshot.current.pollRetry)
                : value,
          },
        }),
    }),
    [context, send],
  );
  const [doc, setDoc] = useState<Document | null>(null);
  const setRoot = useCallback((node: HTMLElement | null) => {
    if (node) {
      life.current.hidden = node.ownerDocument.visibilityState === 'hidden';
      setDoc(node.ownerDocument);
    }
  }, []);
  return { state, context, actions, doc, setRoot };
}
