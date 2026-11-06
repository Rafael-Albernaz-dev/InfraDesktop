import type { PerformFn,useAccessSession } from './access-session';
import type { Intent } from './microcopy';
import { message } from './microcopy';
import type { Result } from './presentation';

export function createPerformAction(
  session: ReturnType<typeof useAccessSession>,
): PerformFn {
  return async function perform(
    call: () => Promise<Result>,
    after?: (r: Result) => void,
    applyResult = true,
    intent: Intent = 'general',
  ) {
    if (session.workingRef.current) return false;
    session.workingRef.current = true;
    session.setBusy(true);
    session.setError('');
    const generation = ++session.epochRef.current;
    const operationSurface = session.surfaceRef.current;
    try {
      const r = await call();
      if (generation !== session.epochRef.current) return false;
      if (intent === 'reset' && ['RESET_OUTCOME_UNKNOWN', 'RESET_VERIFICATION_REQUIRED'].includes(r.code))
        session.setResetVerification(true);
      if (intent === 'login' && ['OK', 'UNAUTHENTICATED'].includes(r.code))
        session.setResetVerification(false);
      if (intent === 'renew') {
        session.setRecoveryUncertain((prev) =>
          r.code === 'OK'
            ? false
            : prev ||
              ['DEADLINE_EXCEEDED', 'SERVICE_UNAVAILABLE'].includes(r.code),
        );
        if (r.code !== 'OK') session.securityLockedRef.current = false;
      }
      if (r.code !== 'OK') {
        if (operationSurface === session.surfaceRef.current)
          session.setError(message(r.code, intent, r.error_reason));
        return false;
      }
      const ownsSurface = operationSurface === session.surfaceRef.current;
      if (applyResult) session.accept(r);
      if (r.recovery)
        session.setRecovery(
          r.recovery.match(/.{1,8}/g)?.join('-') || r.recovery,
        );
      if (ownsSurface) after?.(r);
      return true;
    } catch {
      if (intent === 'renew') {
        session.setRecoveryUncertain(true);
        session.securityLockedRef.current = false;
      }
      if (
        generation === session.epochRef.current &&
        operationSurface === session.surfaceRef.current
      )
        session.setError(message('SERVICE_UNAVAILABLE', intent));
      return false;
    } finally {
      session.workingRef.current = false;
      session.setBusy(false);
    }
  };
}
