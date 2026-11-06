import type { MutableRefObject } from 'react';
import type { ReadEvent, ReadState } from './pppoe-read-machine';
import type { PPReadPanelProps } from './pppoe-read-props';
import type { PPReadResult, PPResult } from './presentation';
export function createReadLifecycle() {
  return {
    disposed: false,
    automaticStarted: false,
    generation: 0,
    jobGeneration: 0,
    activeJob: '',
    privateJob: '',
    operation: '',
    liveStarted: 0,
    requestedAt: 0,
    sampleClock: 0,
    lastSample: '',
    history: undefined as PPReadResult | undefined,
    window: undefined as PPReadResult | undefined,
    renewalTimer: undefined as ReturnType<typeof setTimeout> | undefined,
    pollTimer: undefined as ReturnType<typeof setTimeout> | undefined,
    hidden: false,
    operatorStopped: false,
    retries: 0,
  };
}
export type ReadLifecycle = ReturnType<typeof createReadLifecycle>;
export type ReadContext = {
  life: MutableRefObject<ReadLifecycle>;
  latest: MutableRefObject<PPReadPanelProps>;
  state: MutableRefObject<ReadState>;
  send: (e: ReadEvent) => void;
  start: (mode: string, preserve?: boolean) => Promise<void>;
  schedule: (delay: number) => void;
  failed: (code: string) => Promise<void>;
};
export function validRead(p: PPReadPanelProps, r: PPResult) {
  return r.profile_id === p.profile && r.activation_id === p.activation;
}
export function currentRead(c: ReadContext, g: number) {
  return !c.life.current.disposed && c.life.current.generation === g;
}
