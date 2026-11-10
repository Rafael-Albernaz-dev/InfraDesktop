import { useContext, useLayoutEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Button } from '../../shared/ui/Button';
import { Dialog } from '../../shared/ui/Dialog';
import { NavigationGuard } from '../../shared/navigation-guard';
import { DraftProtection } from '../equipment/equipment-draft';
import type { DraftState } from '../equipment/equipment-draft';

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
  showClose = true,
}: {
  title: string;
  close: () => void;
  children: ReactNode;
  showClose?: boolean;
}) {
  const guard = useRef<() => DraftState>(() => ({
    dirty: false,
    blocked: false,
  }));
  const [confirm, setConfirm] = useState(false);
  const pendingAction = useRef(close);
  const navigationGuard = useContext(NavigationGuard);
  const attempt = (action: () => void) => {
    const state = guard.current();
    if (state.blocked) return;
    pendingAction.current = action;
    if (state.dirty) setConfirm(true);
    else action();
  };
  const requestClose = () => attempt(close);
  useLayoutEffect(() => {
    if (!navigationGuard) return;
    navigationGuard((action) =>
      attempt(() => {
        close();
        action();
      }),
    );
    return () => {
      navigationGuard();
    };
  });
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
        showClose={showClose}
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
