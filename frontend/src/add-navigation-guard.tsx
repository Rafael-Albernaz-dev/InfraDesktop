import {
  useCallback,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { NavigationGuard } from './navigation-guard';
import { Button } from './ui/Button';
import { Dialog } from './ui/Dialog';

export function useAddNavigationGuard(
  dirty: boolean,
  blocked: boolean,
  enabled = true,
) {
  const register = useContext(NavigationGuard);
  const current = useRef({ dirty, blocked });
  useLayoutEffect(() => {
    current.current = { dirty, blocked };
  }, [dirty, blocked]);
  const [confirm, setConfirm] = useState(false);
  const pending = useRef<() => void>(() => {});
  const attempt = useCallback((action: () => void) => {
    if (current.current.blocked) return;
    if (!current.current.dirty) {
      action();
      return;
    }
    pending.current = action;
    setConfirm(true);
  }, []);
  useLayoutEffect(() => {
    if (!enabled) return;
    register?.(attempt);
    return () => register?.();
  });
  const dialog = confirm && (
    <Dialog
      portal
      title="Descartar alterações?"
      close={() => setConfirm(false)}
      dismissOnBackdrop={false}
    >
      <p>As alterações deste formulário serão perdidas.</p>
      <div className="dialog-actions">
        <Button onClick={() => setConfirm(false)}>Continuar editando</Button>
        <Button
          kind="primary"
          onClick={() => {
            setConfirm(false);
            pending.current();
          }}
        >
          Descartar alterações
        </Button>
      </div>
    </Dialog>
  );
  return { attempt, dialog };
}
