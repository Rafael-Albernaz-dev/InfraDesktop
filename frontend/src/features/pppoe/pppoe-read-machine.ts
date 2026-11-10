import type { PPReadResult } from '../../platform/wails/presentation';
export type ReadPhase =
  | 'parado'
  | 'solicitado'
  | 'iniciado'
  | 'amostra'
  | 'concluído'
  | 'falhou'
  | 'renovando'
  | 'pausado';
export const initialReadState = {
  phase: 'parado' as ReadPhase,
  job: '',
  starting: false,
  pending: false,
  cancelling: false,
  pageCursor: '',
  next: '',
  pageBusy: false,
  resultJob: '',
  error: '',
  failureReason: '',
  result: undefined as PPReadResult | undefined,
  scope: 'bandwidth',
  pollRetry: 0,
  statusLatency: undefined as number | undefined,
  firstStatus: undefined as number | undefined,
  deadlineReached: false,
  sampleAge: 0,
  stoppedTraffic: false,
  renewing: false,
};
export type ReadState = typeof initialReadState;
export type ReadEvent =
  | { type: 'solicitado'; scope: string; preserve: boolean }
  | { type: 'iniciado'; job: string }
  | { type: 'amostra'; result: PPReadResult; next: string }
  | { type: 'concluído' }
  | { type: 'falhou'; error: string; reason?: string; pending?: boolean }
  | { type: 'parado' | 'pausado' | 'renovando' }
  | { type: 'metadata'; values: Partial<ReadState> };
export function readReducer(s: ReadState, e: ReadEvent): ReadState {
  switch (e.type) {
    case 'solicitado':
      return requested(s, e);
    case 'iniciado':
      return {
        ...s,
        phase: e.type,
        starting: false,
        pending: false,
        job: e.job,
        resultJob: e.job,
      };
    case 'amostra':
      return { ...s, phase: e.type, result: e.result, next: e.next, error: '' };
    case 'concluído':
      return {
        ...s,
        phase: e.type,
        job: '',
        pending: false,
        starting: false,
        deadlineReached: false,
      };
    case 'falhou':
      return {
        ...s,
        phase: e.type,
        error: e.error,
        failureReason: e.reason || '',
        starting: false,
        pending: e.pending ?? s.pending,
      };
    case 'renovando':
      return {
        ...s,
        phase: e.type,
        renewing: true,
        job: '',
        pending: false,
        starting: false,
      };
    case 'parado':
      return stopped(s);
    case 'pausado':
      return {
        ...stopped(s),
        phase: e.type,
        stoppedTraffic: s.stoppedTraffic,
        error: s.error,
      };
    case 'metadata':
      return { ...s, ...e.values };
  }
}

function requested(
  s: ReadState,
  e: Extract<ReadEvent, { type: 'solicitado' }>,
): ReadState {
  return {
    ...s,
    phase: e.type,
    scope: e.scope,
    starting: true,
    pending: true,
    error: '',
    failureReason: '',
    stoppedTraffic: false,
    renewing: false,
    next: '',
    pageCursor: '',
    resultJob: '',
    deadlineReached: false,
    sampleAge: 0,
    statusLatency: undefined,
    firstStatus: undefined,
    result: e.preserve ? s.result : undefined,
  };
}

function stopped(s: ReadState): ReadState {
  return {
    ...s,
    phase: 'parado',
    renewing: false,
    job: '',
    pending: false,
    starting: false,
    cancelling: false,
    stoppedTraffic: true,
    deadlineReached: false,
    error: '',
  };
}
