import { createContext } from 'react';

export type SessionRouteTarget = {
  device: string;
  observation: string;
  row: string;
  login: string;
};
export type SessionNavigation = {
  active: boolean;
  target?: SessionRouteTarget;
  open: (target: SessionRouteTarget) => void;
  back: () => void;
  leave: () => void;
};
export const SessionRoute = createContext<SessionNavigation | undefined>(
  undefined,
);
