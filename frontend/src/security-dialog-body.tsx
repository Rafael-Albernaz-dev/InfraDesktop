import type { AccessErrorKind } from './access-error-state';
import { PerformFn } from './access-session';
import { access } from './presentation';
import { Security } from './security';

export type SecurityDialogBodyProps = {
  recovery: string;
  busy: boolean;
  error: string;
  errorKind: AccessErrorKind;
  recoveryUncertain: boolean;
  securityLockedRef: React.MutableRefObject<boolean>;
  workingRef: React.MutableRefObject<boolean>;
  perform: PerformFn;
  setRecovery: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  close: () => void;
};
export function SecurityDialogBody(props: SecurityDialogBodyProps) {
  const {
    recovery,
    busy,
    error,
    errorKind,
    recoveryUncertain,
    securityLockedRef,
    workingRef,
    perform,
    setRecovery,
    setRecoveryUncertain,
    close,
  } = props;
  return (
    <Security
      recovery={recovery}
      busy={busy}
      error={error}
      errorKind={errorKind}
      uncertain={recoveryUncertain}
      cancel={close}
      lock={(locked) => {
        securityLockedRef.current = locked;
      }}
      renew={(password) => {
        if (workingRef.current) return;
        const replace = recoveryUncertain;
        securityLockedRef.current = true;
        void perform(
          () => access.renewRecovery(password, replace),
          undefined,
          true,
          'renew',
        );
      }}
      acknowledge={() =>
        void perform(
          () => access.ackRecovery(),
          () => {
            setRecovery('');
            setRecoveryUncertain(false);
            securityLockedRef.current = false;
            close();
          },
        )
      }
    />
  );
}
