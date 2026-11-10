import { createContext } from 'react';

export type SessionRouteTarget = {
  device: string;
  observation: string;
  row: string;
  login: string;
  originDevice?: string;
};
export type SessionNavigation = {
  active: boolean;
  target?: SessionRouteTarget;
  returnOrigin?: string;
  consumeReturnOrigin?: () => void;
  open: (target: SessionRouteTarget) => void;
  back: () => void;
  leave: () => void;
};
export const SessionRoute = createContext<SessionNavigation | undefined>(
  undefined,
);

export type LinkedSessionTarget = {
  deviceID: string;
  observation: string;
  row: string;
  originDevice?: string;
};
