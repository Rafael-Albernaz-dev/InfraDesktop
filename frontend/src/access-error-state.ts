import type { Intent } from './microcopy';
export type AccessErrorKind = 'general' | 'incorrect-password';
export function accessErrorKind(
  code: string,
  intent: Intent,
  reason?: string,
): AccessErrorKind {
  return code === 'UNAUTHENTICATED' &&
    (intent === 'renew' || intent === 'password') &&
    reason === 'PASSWORD_INCORRECT'
    ? 'incorrect-password'
    : 'general';
}
