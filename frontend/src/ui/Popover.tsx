import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Button } from './Button';

export function Popover({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const outside = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        setOpen(false);
        root.current?.querySelector('button')?.focus();
      }
    };
    const owner = root.current?.ownerDocument;
    owner?.addEventListener('mousedown', outside);
    owner?.addEventListener('keydown', key);
    return () => {
      owner?.removeEventListener('mousedown', outside);
      owner?.removeEventListener('keydown', key);
    };
  }, [open]);
  return (
    <div className="inventory-popover" ref={root}>
      <Button
        aria-haspopup="true"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {label}
      </Button>
      {open && (
        <div className="dd inventory-menu" role="menu" aria-label={label}>
          {children}
        </div>
      )}
    </div>
  );
}
