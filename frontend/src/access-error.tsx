import type { AccessErrorKind } from './access-error-state';
export function AccessError({
  text,
  kind,
}: {
  text: string;
  kind: AccessErrorKind;
}) {
  return (
    <p
      role="alert"
      className={
        kind === 'incorrect-password'
          ? 'inline-error inline-error-danger'
          : 'inline-error'
      }
    >
      {text}
    </p>
  );
}
