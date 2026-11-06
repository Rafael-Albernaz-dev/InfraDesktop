import type { ReadContext } from './pppoe-read-lifecycle';
import type { PPReadPanelProps } from './pppoe-read-props';
import { pppoeReadAPI as api } from './presentation';
export function prepareRead(c: ReadContext, mode: string, preserve: boolean) {
  const l = c.life.current;
  if (l.privateJob)
    void api
      .cancel(
        c.latest.current.profile,
        c.latest.current.activation,
        l.privateJob,
      )
      .catch(() => {});
  l.privateJob = '';
  l.operation = crypto.randomUUID();
  l.generation++;
  l.requestedAt = performance.now();
  l.liveStarted = l.requestedAt;
  l.sampleClock = l.liveStarted;
  l.lastSample = '';
  if (!preserve) {
    l.history = undefined;
    l.retries = 0; l.terminalStopped = false;
  }
  if (mode === 'traffic') l.window = undefined;
  l.operatorStopped = false;
  c.send({ type: 'solicitado', scope: mode, preserve });
  return l.generation;
}
export function readTarget(p: PPReadPanelProps, mode: string) {
  if (['top', 'health', 'operations'].includes(mode))
    return { obs: '', rows: [] };
  if (['log', 'secret'].includes(mode))
    return { obs: p.observation, rows: p.logRow ? [p.logRow] : [] };
  return { obs: p.observation, rows: p.rows };
}
