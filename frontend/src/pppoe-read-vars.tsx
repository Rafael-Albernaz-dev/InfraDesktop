import { useState } from 'react';
import type { PPReadResult } from './presentation';

export function usePPReadStateVars() {
  const [job, setJob] = useState('');
  const [starting, setStarting] = useState(false);
  const [pending, setPending] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [pageCursor, setPageCursor] = useState('');
  const [next, setNext] = useState('');
  const [pageBusy, setPageBusy] = useState(false);
  const [resultJob, setResultJob] = useState('');
  const [error, setError] = useState('');
  const [failureReason, setFailureReason] = useState('');
  const [result, setResult] = useState<PPReadResult>();
  const [scope, setScope] = useState('bandwidth');
  const [pollRetry, retryPoll] = useState(0);
  const [statusLatency, setStatusLatency] = useState<number>();
  const [firstStatus, setFirstStatus] = useState<number>();
  const [deadlineReached, setDeadlineReached] = useState(false);
  const [sampleAge, setSampleAge] = useState(0);
  const [stoppedTraffic, setStoppedTraffic] = useState(false);
  const [renewing, setRenewing] = useState(false);
  return {
    job,
    setJob,
    starting,
    setStarting,
    pending,
    setPending,
    cancelling,
    setCancelling,
    pageCursor,
    setPageCursor,
    next,
    setNext,
    pageBusy,
    setPageBusy,
    resultJob,
    setResultJob,
    error,
    setError,
    failureReason,
    setFailureReason,
    result,
    setResult,
    scope,
    setScope,
    pollRetry,
    retryPoll,
    statusLatency,
    setStatusLatency,
    firstStatus,
    setFirstStatus,
    deadlineReached,
    setDeadlineReached,
    sampleAge,
    setSampleAge,
    stoppedTraffic,
    setStoppedTraffic,
    renewing,
    setRenewing,
  };
}
export type ReadState = ReturnType<typeof usePPReadStateVars>;
