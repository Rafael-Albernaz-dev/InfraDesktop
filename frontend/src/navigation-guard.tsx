import { createContext } from 'react';
export type NavigationAttempt = (action: () => void) => void;
export const NavigationGuard = createContext<
  ((attempt?: NavigationAttempt) => void) | null
>(null);
