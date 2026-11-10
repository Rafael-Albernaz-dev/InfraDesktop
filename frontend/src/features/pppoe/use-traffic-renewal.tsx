import { useEffect } from 'react';
import type { ReadContext } from './pppoe-read-lifecycle';
import { canResumeTraffic } from './pppoe-read-lifecycle';
import { releaseRead } from './pppoe-read-cancel';
const retryDelays = [5000, 15000, 30000];
export function scheduleTraffic(c: ReadContext, delay: number) {
  const l = c.life.current;
  clearTimeout(l.renewalTimer);
  if (!canResumeTraffic(c)) return;
  c.send({ type: 'renovando' });
  l.renewalTimer = setTimeout(() => {
    if (!canResumeTraffic(c)) return;
    void c.start('traffic', true);
  }, delay);
}
export async function trafficFailed(c: ReadContext, code: string) {
  const l = c.life.current,
    g = l.generation;
  if (
    !['BUSY', 'DEADLINE_EXCEEDED', 'SERVICE_UNAVAILABLE'].includes(code) ||
    l.retries >= retryDelays.length
  )
    l.terminalStopped = true;
  try {
    if (!(await releaseRead(c))) return;
  } catch {
    if (l.generation === g + 1)
      c.send({
        type: 'falhou',
        error: 'Falha na leitura; cancelamento não confirmado.',
      });
    return;
  }
  c.send({ type: 'metadata', values: { job: '', pending: false } });
  if (!canResumeTraffic(c)) return;
  scheduleTraffic(c, retryDelays[l.retries++]);
}
async function pauseTraffic(c: ReadContext) {
  c.life.current.history = c.life.current.window || c.life.current.history;
  try {
    if (await releaseRead(c)) {
      c.send({ type: 'pausado' });
      if (canResumeTraffic(c)) void c.start('traffic', true);
    }
  } catch {
    c.send({
      type: 'falhou',
      error: 'Falha na leitura; cancelamento não confirmado.',
    });
  }
}
function syncVisibility(c: ReadContext, doc: Document, resume: boolean) {
  const l = c.life.current;
  l.hidden = doc.visibilityState === 'hidden';
  if (l.hidden) void pauseTraffic(c);
  else if (resume && canResumeTraffic(c)) void c.start('traffic', true);
}
export function useTrafficRenewal(
  c: ReadContext,
  doc: Document | null,
  enabled: boolean,
) {
  useEffect(() => {
    if (!enabled || !doc) return;
    const changed = () => syncVisibility(c, doc, true);
    syncVisibility(c, doc, false);
    doc.addEventListener('visibilitychange', changed);
    return () => doc.removeEventListener('visibilitychange', changed);
  }, [c, doc, enabled]);
}
