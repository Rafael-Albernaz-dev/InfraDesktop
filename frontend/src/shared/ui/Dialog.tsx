import type { ReactNode } from 'react';
import { useContext, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { Icon } from './icons';
import { Button } from './Button';
import { DialogHost } from './dialog-host';
const FOCUSABLE =
  'input:not(:disabled):not([type="hidden"]),button:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex="0"]';

function trapDialogKey(
  e: React.KeyboardEvent,
  ref: React.RefObject<HTMLElement | null>,
  close: () => void,
) {
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
    close();
    return;
  }
  if (e.key !== 'Tab') return;
  const f = Array.from(
    ref.current?.querySelectorAll<HTMLElement>(FOCUSABLE) || [],
  ).filter((el) => !el.closest('[hidden],[aria-hidden="true"]'));
  if (!f.length) {
    e.preventDefault();
    return;
  }
  const active = ref.current?.ownerDocument.activeElement;
  if (!f.some((el) => el === active)) {
    e.preventDefault();
    (e.shiftKey ? f.at(-1) : f[0])?.focus();
  } else if (e.shiftKey && active === f[0]) {
    e.preventDefault();
    f.at(-1)?.focus();
  } else if (!e.shiftKey && active === f.at(-1)) {
    e.preventDefault();
    f[0].focus();
  }
}

export function Dialog({
  title,
  children,
  close,
  portal = false,
  dismissOnBackdrop = true,
  inactive = false,
  showClose = true,
}: {
  title: string;
  children: ReactNode;
  close: () => void;
  portal?: boolean;
  dismissOnBackdrop?: boolean;
  inactive?: boolean;
  showClose?: boolean;
}) {
  const host = useContext(DialogHost);
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    const el = ref.current;
    const before = el?.ownerDocument.activeElement as HTMLElement | null;
    const f = Array.from(
      el?.querySelectorAll<HTMLElement>(FOCUSABLE) || [],
    ).filter((node) => !node.closest('[hidden],[aria-hidden="true"]'));
    if (showClose) f[0]?.focus();
    else el?.querySelector('h2')?.focus();
    return () => {
      if (before?.isConnected) before.focus();
    };
  }, [showClose, title]);
  const content = (
    <div
      className="modal-backdrop dlg-bg"
      onClick={(e) => {
        if (dismissOnBackdrop && !inactive && e.target === e.currentTarget)
          close();
      }}
      onKeyDown={(e) => !inactive && trapDialogKey(e, ref, close)}
    >
      <section
        ref={ref}
        className="dialog dlg"
        role="dialog"
        inert={inactive}
        aria-hidden={inactive || undefined}
        aria-modal="true"
        aria-label={title}
      >
        {showClose ? (
          <div className="row between">
            <h2>{title}</h2>
            <Button
              kind="flat dialog-close"
              onClick={close}
              label="Fechar painel"
            >
              <Icon name="XMark" />
            </Button>
          </div>
        ) : (
          <h2 tabIndex={-1}>{title}</h2>
        )}
        {children}
      </section>
    </div>
  );
  return portal && host ? createPortal(content, host) : content;
}
