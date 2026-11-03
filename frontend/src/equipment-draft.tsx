import {
  createContext,
  useContext,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import type { ReactNode } from 'react';
import { Button, Dialog } from './desktop';

export type DraftState = { dirty: boolean; blocked: boolean };

export const DraftProtection = createContext<{
  register: (read: () => DraftState) => void;
  cancel: () => void;
  attempt: (action: () => void) => void;
} | null>(null);

function DiscardConfirmDialog({
  close,
  confirmDiscard,
}: {
  close: () => void;
  confirmDiscard: () => void;
}) {
  return (
    <Dialog
      portal
      title="Descartar alterações?"
      close={close}
      dismissOnBackdrop={false}
    >
      <p>As alterações deste formulário serão perdidas.</p>
      <div className="dialog-actions">
        <Button onClick={close}>Continuar editando</Button>
        <Button kind="primary" onClick={confirmDiscard}>
          Descartar alterações
        </Button>
      </div>
    </Dialog>
  );
}

export function ProtectedDialog({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: ReactNode;
}) {
  const guard = useRef<() => DraftState>(() => ({
    dirty: false,
    blocked: false,
  }));
  const [confirm, setConfirm] = useState(false);
  const pendingAction = useRef(close);
  const attempt = (action: () => void) => {
    const state = guard.current();
    if (state.blocked) return;
    pendingAction.current = action;
    if (state.dirty) setConfirm(true);
    else action();
  };
  const requestClose = () => attempt(close);
  return (
    <DraftProtection.Provider
      value={{
        register: (read) => {
          guard.current = read;
        },
        cancel: requestClose,
        attempt,
      }}
    >
      <Dialog
        portal
        title={title}
        close={requestClose}
        dismissOnBackdrop={false}
        inactive={confirm}
      >
        {children}
      </Dialog>
      {confirm && (
        <DiscardConfirmDialog
          close={() => setConfirm(false)}
          confirmDiscard={() => {
            if (!guard.current().blocked) {
              setConfirm(false);
              pendingAction.current();
            }
          }}
        />
      )}
    </DraftProtection.Provider>
  );
}

export function useDraftProtection(read: () => DraftState) {
  const protection = useContext(DraftProtection);
  useLayoutEffect(() => {
    protection?.register(read);
  });
  return protection?.cancel;
}

export function useFieldErrors(disabled: boolean) {
  const prefix = useId();
  const ref = useRef<HTMLFormElement>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  useLayoutEffect(() => {
    const field = Object.keys(errors)[0];
    if (field) {
      const control = ref.current?.querySelector<HTMLElement>(
        `[name="${field}"]:not(:disabled)`,
      );
      control?.closest('details')?.setAttribute('open', '');
      control?.focus();
    }
  }, [errors, disabled]);
  const attrs = (name: string) => ({
    name,
    'aria-invalid': errors[name] ? true : undefined,
    'aria-describedby': errors[name] ? `${prefix}-${name}` : undefined,
  });
  const message = (name: string) =>
    errors[name] ? (
      <span id={`${prefix}-${name}`} className="field-error">
        {errors[name]}
      </span>
    ) : null;
  return { ref, errors, setErrors, attrs, message };
}
