import type { ReactNode } from 'react';
import { createContext, useContext } from 'react';
import { createPortal } from 'react-dom';

export const HeaderActionHost = createContext<HTMLElement | null>(null);
export function HeaderAction({ children }: { children: ReactNode }) {
  const host = useContext(HeaderActionHost);
  return host ? createPortal(children, host) : children;
}
