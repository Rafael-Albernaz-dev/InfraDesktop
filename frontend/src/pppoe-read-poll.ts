import type { ReadContext } from './pppoe-read-lifecycle';
import { currentRead, validRead } from './pppoe-read-lifecycle';
import { ppReadMessage } from './pppoe-context-actions';
import { pppoeReadAPI as api, type PPResult } from './presentation';
import { mergeTrafficWindows } from './pppoe-traffic-windows';
function acceptSample(c: ReadContext, r: PPResult) {
  if (!r.data?.read) return;
  const l = c.life.current,
    read = r.data.read;
  if (l.lastSample !== read.ended_at) {
    l.lastSample = read.ended_at;
    l.sampleClock = performance.now();
    c.send({ type: 'metadata', values: { sampleAge: 0 } });
  }
  const result =
    c.state.current.scope === 'traffic'
      ? mergeTrafficWindows(l.history, read)
      : read;
  if (c.state.current.scope === 'traffic') {
    l.window = result;
  }
  c.send({ type: 'amostra', result, next: r.data.next_cursor || '' });
}
function completeRead(c: ReadContext, r: PPResult) {
  const job = r.data!.job!,
    l = c.life.current,
    scope = c.state.current.scope;
  l.activeJob = '';
  c.send({ type: 'concluído' });
  if (job.code !== 'OK' || job.state !== 'SUCCEEDED') {
    c.send({
      type: 'falhou',
      error: ppReadMessage(job.code),
      reason: r.error?.reason,
    });
    if (scope === 'traffic') void c.failed(job.code);
  } else if (scope === 'operations') c.latest.current.onRead?.();
  else if (scope === 'traffic' && l.window) {
    l.history = l.window;
    l.retries = 0;
    if (l.operatorStopped) c.send({ type: 'parado' });
    else c.schedule(Math.max(0, 55000 - (performance.now() - l.liveStarted)));
  } else if (scope === 'traffic')
    c.send({ type: 'falhou', error: 'Leitura de tráfego sem amostras.' });
}
async function pollFailure(c: ReadContext, code: string) {
  c.send({ type: 'falhou', error: ppReadMessage(code) });
  if (c.state.current.scope === 'traffic') await c.failed(code);
}
export async function pollRead(c: ReadContext, g: number) {
  const p = c.latest.current,
    id = c.life.current.activeJob;
  if (!id || !currentRead(c, g)) return;
  try {
    const before = performance.now(),
      r = await api.status(p.profile, p.activation, id);
    if (!currentRead(c, g)) return;
    c.send({
      type: 'metadata',
      values: { statusLatency: performance.now() - before },
    });
    if (!validRead(p, r) || r.code !== 'OK' || !r.data?.job) {
      await pollFailure(c, validRead(p, r) ? r.code : 'PROFILE_CHANGED');
      return;
    }
    c.send({
      type: 'metadata',
      values: {
        error: '',
        firstStatus:
          c.state.current.firstStatus ??
          performance.now() - c.life.current.requestedAt,
      },
    });
    acceptSample(c, r);
    if (r.data.job.state === 'RUNNING')
      c.life.current.pollTimer = setTimeout(() => void pollRead(c, g), 350);
    else completeRead(c, r);
  } catch {
    if (currentRead(c, g)) {
      c.send({
        type: 'falhou',
        error:
          'Estado da leitura indisponível. Consulte o estado ou cancele a solicitação.',
      });
      if (c.state.current.scope === 'traffic')
        await c.failed('SERVICE_UNAVAILABLE');
    }
  }
}
