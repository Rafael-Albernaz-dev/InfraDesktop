import { createContext } from 'react';
const noop = () => {};
export const NetworkPulse = createContext<{
  pulse: string;
  publish: (pulse: string) => void;
  register: () => () => void;
}>({ pulse: '', publish: noop, register: () => noop });
