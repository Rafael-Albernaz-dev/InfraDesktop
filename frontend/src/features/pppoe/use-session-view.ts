import { useCallback, useState } from 'react';
import { accessErrorKind } from '../access/access-error-state';
import type { AccessErrorKind } from '../access/access-error-state';
import type { Result } from '../../platform/wails/presentation';
import type { Intent } from '../access/microcopy';
import { message } from '../access/microcopy';
export function useSessionView(initial: Result) {
  const [view, setView] = useState(initial);
  const [connected, setConnected] = useState(false);
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState({
    error: '',
    errorKind: 'general' as AccessErrorKind,
  });
  const setError = useCallback(
    (error: string) => setFeedback({ error, errorKind: 'general' }),
    [],
  );
  const setOperationError = useCallback(
    (r: Result, intent: Intent) =>
      setFeedback({
        error: message(r.code, intent, r.error_reason),
        errorKind: accessErrorKind(r.code, intent, r.error_reason),
      }),
    [],
  );
  const [recovery, setRecovery] = useState('');
  const [resetVerification, setResetVerification] = useState(false);
  const [dialog, setDialog] = useState('');
  const [recoveryUncertain, setRecoveryUncertain] = useState(false);
  return {
    view,
    setView,
    connected,
    setConnected,
    busy,
    setBusy,
    ...feedback,
    setError,
    setOperationError,
    recovery,
    setRecovery,
    resetVerification,
    setResetVerification,
    dialog,
    setDialog,
    recoveryUncertain,
    setRecoveryUncertain,
  };
}
