import { useEffect, useRef } from 'react';
import { usePPReadStateVars } from './pppoe-read-vars';
import { usePPReadCleanup } from './pppoe-read-cleanup';
import type { ReadState } from './pppoe-read-vars';
export type { ReadState } from './pppoe-read-vars';
import type { Device } from './equipment-api';
import { ppReadMessage } from './pppoe-context';
export { ppReadMessage } from './pppoe-context';
import { RadioCodes, RadioModes } from './radio-contract';
import type { PPReadResult, PPResult } from './presentation';
import { pppoeReadAPI as readAPI } from './presentation';
import { mergeTrafficWindows } from './pppoe-traffic-windows';
export type PPReadPanelProps = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  rows: string[];
  blocked?: boolean;
  allowTop?: boolean;
  logRow?: string;
  health?: boolean;
  monitoring?: boolean;
  onRead?: () => void;
  summaryOnly?: boolean;
  traffic?: boolean;
  bandwidthOnly?: boolean;
  pageBandwidth?: boolean;
  panel?: boolean;
  evidence?: 'log' | 'secret';
  actionsOnly?: boolean;
  healthView?: 'pools' | 'uplinks';
};
const READ_REJECTED = [
  'BUSY',
  'REVISION_CONFLICT',
  'INVALID_ARGUMENT',
  'PROFILE_CHANGED',
  'UNAUTHENTICATED',
  'DEVICE_INACTIVE',
  'TRUST_REQUIRED',
  'VAULT_UNAVAILABLE',
  'OBSERVATION_UNAVAILABLE',
  'OBSERVATION_INCOMPATIBLE',
  'LIMIT_EXCEEDED',
  RadioCodes.UNSUPPORTED,
];



function resolveStartTarget(
  mode: string,
  observation: string,
  logRow?: string,
  rows?: string[],
) {
  const obs =
    mode === 'top' || mode === 'health' || mode === 'operations'
      ? ''
      : observation;
  const targetRows =
    mode === 'top' || mode === 'health' || mode === 'operations'
      ? []
      : mode === 'log' || mode === 'secret'
        ? logRow
          ? [logRow]
          : []
        : rows || [];
  return { obs, targetRows };
}

function usePPReadStartAction(p: {
  s: ReadState;
  pProps: PPReadPanelProps;
  generation: React.MutableRefObject<number>;
  activeJob: React.MutableRefObject<string>;
  privateResultJob: React.MutableRefObject<string>;
  pendingOperation: React.MutableRefObject<string>;
  liveStarted: React.MutableRefObject<number>;
  requestedAt: React.MutableRefObject<number>;
  sampleClock: React.MutableRefObject<number>;
  lastSample: React.MutableRefObject<string>;
  trafficHistory: React.MutableRefObject<PPReadResult | undefined>;
  trafficWindow: React.MutableRefObject<PPReadResult | undefined>;
  valid: (r: PPResult) => boolean;
}) {
  const {
    s,
    pProps,
    generation,
    activeJob,
    privateResultJob,
    pendingOperation,
    liveStarted,
    requestedAt,
    sampleClock,
    lastSample,
    trafficHistory,
    trafficWindow,
    valid,
  } = p;
  return async (mode: string, renewing = false) => {
    requestedAt.current = performance.now();
    s.setStatusLatency(undefined);
    s.setFirstStatus(undefined);
    if (pendingOperation.current || activeJob.current) return;
    if (privateResultJob.current) {
      void readAPI
        .cancel(pProps.profile, pProps.activation, privateResultJob.current)
        .catch(() => {});
      privateResultJob.current = '';
    }
    const g = ++generation.current;
    const operation = crypto.randomUUID();
    pendingOperation.current = operation;
    s.setPending(true);
    s.setScope(mode);
    s.setStoppedTraffic(false);
    s.setStarting(true);
    s.setError('');
    s.setFailureReason('');
    if (!renewing) {
      trafficHistory.current = undefined;
      s.setResult(undefined);
    }
    if (mode === 'traffic') trafficWindow.current = undefined;
    s.setNext('');
    s.setPageCursor('');
    s.setResultJob('');
    s.setDeadlineReached(false);
    liveStarted.current = performance.now();
    sampleClock.current = liveStarted.current;
    lastSample.current = '';
    s.setSampleAge(0);
    const { obs, targetRows } = resolveStartTarget(
      mode,
      pProps.observation,
      pProps.logRow,
      pProps.rows,
    );
    try {
      const r = await readAPI.start(
        pProps.profile,
        pProps.activation,
        pProps.device.id,
        operation,
        mode,
        obs,
        targetRows,
        pProps.device.revision,
      );
      if (g !== generation.current) {
        if (valid(r) && r.data?.job)
          void readAPI
            .cancel(pProps.profile, pProps.activation, r.data.job.id)
            .catch(() => {});
        return;
      }
      if (!valid(r) || r.code !== 'OK' || !r.data?.job) {
        if (READ_REJECTED.includes(r.code)) {
          pendingOperation.current = '';
          s.setPending(false);
        }
        s.setError(ppReadMessage(r.code));
        return;
      }
      pendingOperation.current = '';
      s.setPending(false);
      activeJob.current = r.data.job.id;
      if (mode === 'secret' || mode === 'traffic')
        privateResultJob.current = r.data.job.id;
      s.setJob(r.data.job.id);
      s.setResultJob(r.data.job.id);
    } catch {
      if (g === generation.current) {
        s.setError(
          'Não foi possível confirmar a solicitação. Cancele a solicitação pendente antes de iniciar outra.',
        );
      }
    } finally {
      if (g === generation.current) s.setStarting(false);
    }
  };
}

function usePPReadPollEffect(p: {
  s: ReadState;
  pProps: PPReadPanelProps;
  generation: React.MutableRefObject<number>;
  activeJob: React.MutableRefObject<string>;
  requestedAt: React.MutableRefObject<number>;
  sampleClock: React.MutableRefObject<number>;
  lastSample: React.MutableRefObject<string>;
  liveStarted: React.MutableRefObject<number>;
  trafficHistory: React.MutableRefObject<PPReadResult | undefined>;
  trafficWindow: React.MutableRefObject<PPReadResult | undefined>;
  renewalTimer: React.MutableRefObject<ReturnType<typeof setTimeout> | undefined>;
  restartTraffic: React.MutableRefObject<() => void>;
  blocked: React.MutableRefObject<boolean>;
}) {
  const {
    s,
    pProps,
    generation,
    activeJob,
    requestedAt,
    sampleClock,
    lastSample,
    liveStarted,
    trafficHistory,
    trafficWindow,
    renewalTimer,
    restartTraffic,
    blocked,
  } = p;
  useEffect(() => {
    if (!s.job) return;
    let alive = true;
    const g = generation.current;
    let timer: ReturnType<typeof setTimeout>;
    const cancelFailedTraffic = () => {
      if (s.scope !== 'traffic') return;
      void readAPI.cancel(pProps.profile, pProps.activation, s.job).then((response) => {
        if (!alive || g !== generation.current) return;
        if (response.code === 'OK' && response.profile_id === pProps.profile && response.activation_id === pProps.activation) {
          activeJob.current = '';
          s.setJob('');
        } else s.setError('Falha na leitura; cancelamento não confirmado.');
      }).catch(() => {
        if (alive && g === generation.current) s.setError('Falha na leitura; cancelamento não confirmado.');
      });
    };
    const poll = async () => {
      try {
        const beforeStatus = performance.now();
        const r = await readAPI.status(
          pProps.profile,
          pProps.activation,
          s.job,
        );
        if (!alive || g !== generation.current) return;
        s.setStatusLatency(performance.now() - beforeStatus);
        if (r.code === 'OK' && r.data?.job)
          s.setFirstStatus(
            (known) => known ?? performance.now() - requestedAt.current,
          );
        if (
          r.profile_id !== pProps.profile ||
          r.activation_id !== pProps.activation ||
          r.code !== 'OK' ||
          !r.data?.job
        ) {
          s.setError(ppReadMessage(r.code));
          cancelFailedTraffic();
          return;
        }
        s.setError('');
        if (r.data.read) {
          if (lastSample.current !== r.data.read.ended_at) {
            lastSample.current = r.data.read.ended_at;
            sampleClock.current = performance.now();
            s.setSampleAge(0);
          }
          const reading = s.scope === 'traffic'
            ? mergeTrafficWindows(trafficHistory.current, r.data.read)
            : r.data.read;
          if (s.scope === 'traffic') trafficWindow.current = reading;
          s.setResult(reading);
          s.setNext(r.data.next_cursor || '');
        }
        if (r.data.job.state === 'RUNNING') {
          timer = setTimeout(poll, 350);
          return;
        }
        activeJob.current = '';
        s.setJob('');
        s.setDeadlineReached(false);
        if (r.data.job.code === 'OK' && s.scope === 'operations')
          pProps.onRead?.();
        if (r.data.job.code !== 'OK' || r.data.job.state !== 'SUCCEEDED') {
          s.setError(ppReadMessage(r.data.job.code));
          s.setFailureReason(r.error?.reason || '');
        } else if (s.scope === 'traffic' && trafficWindow.current) {
          trafficHistory.current = trafficWindow.current;
          s.setRenewing(true);
          const delay = Math.max(0, 55000 - (performance.now() - liveStarted.current));
          renewalTimer.current = setTimeout(() => {
            s.setRenewing(false);
            if (blocked.current) s.setStoppedTraffic(true);
            else restartTraffic.current();
          }, delay);
        } else if (s.scope === 'traffic') {
          s.setError('Leitura de tráfego sem amostras.');
        }
      } catch {
        if (alive && g === generation.current) {
          s.setError(
            'Estado da leitura indisponível. Consulte o estado ou cancele a solicitação.',
          );
          cancelFailedTraffic();
        }
      }
    };
    void poll();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [
    s.job,
    pProps.profile,
    pProps.activation,
    s.pollRetry,
    s.scope,
    pProps.onRead,
  ]);
  useEffect(() => {
    if (s.scope !== RadioModes.live && s.scope !== 'traffic') return;
    if (!s.job && s.scope === RadioModes.live) return;
    const tick = () => {
      const now = performance.now();
      s.setSampleAge(Math.max(0, now - sampleClock.current));
      if (s.scope === RadioModes.live && s.job && now - liveStarted.current >= 60000) {
        s.setDeadlineReached(true);
        clearInterval(timer);
      }
    };
    const timer = setInterval(tick, 1000);
    const initial = setTimeout(tick, 0);
    return () => {
      clearInterval(timer);
      clearTimeout(initial);
    };
  }, [s.job, s.scope]);
}

function usePPReadMutations(p: {
  s: ReadState;
  pProps: PPReadPanelProps;
  generation: React.MutableRefObject<number>;
  activeJob: React.MutableRefObject<string>;
  pendingOperation: React.MutableRefObject<string>;
  renewalTimer: React.MutableRefObject<ReturnType<typeof setTimeout> | undefined>;
  valid: (r: PPResult) => boolean;
}) {
  const { s, pProps, generation, activeJob, pendingOperation, renewalTimer, valid } = p;
  async function cancel() {
    if (s.renewing) {
      clearTimeout(renewalTimer.current);
      s.setRenewing(false);
      s.setStoppedTraffic(true);
      return;
    }
    const id = activeJob.current;
    const op = pendingOperation.current;
    if (!id && !op) return;
    const g = ++generation.current;
    s.setCancelling(true);
    try {
      const r = id
        ? await (pProps.traffic ? readAPI.stop : readAPI.cancel)(
            pProps.profile,
            pProps.activation,
            id,
          )
        : await readAPI.cancelOperation(
            pProps.profile,
            pProps.activation,
            pProps.device.id,
            op,
          );
      if (g !== generation.current) return;
      if (valid(r) && r.code === 'OK') {
        activeJob.current = '';
        pendingOperation.current = '';
        s.setPending(false);
        s.setJob('');
        s.setStarting(false);
        s.setError('');
        if (pProps.traffic) {
          s.setStoppedTraffic(true);
          s.setDeadlineReached(false);
        }
      } else s.setError(ppReadMessage(r.code));
    } catch {
      if (g === generation.current)
        s.setError(
          'Cancelamento não confirmado; o prazo da leitura continua limitado.',
        );
    } finally {
      if (g === generation.current) s.setCancelling(false);
    }
  }
  async function page(cursor: string) {
    const g = generation.current;
    s.setPageBusy(true);
    try {
      const r = await readAPI.page(
        pProps.profile,
        pProps.activation,
        s.resultJob,
        cursor,
      );
      if (g !== generation.current) return;
      if (!valid(r) || r.code !== 'OK' || !r.data?.read) {
        s.setError(ppReadMessage(r.code));
        return;
      }
      s.setResult(r.data.read);
      s.setPageCursor(cursor);
      s.setNext(r.data.next_cursor || '');
      s.setError('');
    } catch {
      if (g === generation.current)
        s.setError('Página da leitura indisponível.');
    } finally {
      if (g === generation.current) s.setPageBusy(false);
    }
  }
  return { cancel, page };
}

export function usePPReadState(pProps: PPReadPanelProps) {
  const s = usePPReadStateVars();
  const generation = useRef(0);
  const activeJob = useRef('');
  const privateResultJob = useRef('');
  const pendingOperation = useRef('');
  const liveStarted = useRef(0);
  const requestedAt = useRef(0);
  const sampleClock = useRef(0);
  const lastSample = useRef('');
  const trafficHistory = useRef<PPReadResult | undefined>(undefined);
  const trafficWindow = useRef<PPReadResult | undefined>(undefined);
  const renewalTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const blocked = useRef(Boolean(pProps.blocked));
  useEffect(() => { blocked.current = Boolean(pProps.blocked); }, [pProps.blocked]);
  usePPReadCleanup({
    profile: pProps.profile,
    activation: pProps.activation,
    deviceId: pProps.device.id,
    generationRef: generation,
    activeJobRef: activeJob,
    privateResultJobRef: privateResultJob,
    pendingOperationRef: pendingOperation,
    renewalTimerRef: renewalTimer,
  });
  const valid = (r: PPResult) =>
    r.profile_id === pProps.profile && r.activation_id === pProps.activation;
  const start = usePPReadStartAction({
    s,
    pProps,
    generation,
    activeJob,
    privateResultJob,
    pendingOperation,
    liveStarted,
    requestedAt,
    sampleClock,
    lastSample,
    trafficHistory,
    trafficWindow,
    valid,
  });
  const restartTraffic = useRef(() => {});
  useEffect(() => { restartTraffic.current = () => { void start('traffic', true); }; }, [start]);
  usePPReadPollEffect({
    s,
    pProps,
    generation,
    activeJob,
    requestedAt,
    sampleClock,
    lastSample,
    liveStarted,
    trafficHistory,
    trafficWindow,
    renewalTimer,
    restartTraffic,
    blocked,
  });
  const { cancel, page } = usePPReadMutations({
    s,
    pProps,
    generation,
    activeJob,
    pendingOperation,
    renewalTimer,
    valid,
  });
  const locked =
    pProps.blocked ||
    s.starting ||
    s.pending ||
    s.renewing ||
    Boolean(s.job) ||
    s.cancelling ||
    s.pageBusy;
  const calculated =
    Boolean(s.job) &&
    (s.scope === RadioModes.live || s.scope === 'traffic') &&
    !s.error &&
    !s.deadlineReached;
  return { ...s, start, cancel, page, locked, calculated };
}
