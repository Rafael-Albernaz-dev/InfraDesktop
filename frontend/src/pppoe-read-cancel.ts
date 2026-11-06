import type { ReadContext } from './pppoe-read-lifecycle';
import { currentRead, validRead } from './pppoe-read-lifecycle';
import { ppReadMessage } from './pppoe-context-actions';
import { pppoeReadAPI as api } from './presentation';
export async function releaseRead(c: ReadContext, operator = false) {
  const l = c.life.current,
    p = c.latest.current,
    id = l.activeJob,
    op = l.operation;
  clearTimeout(l.renewalTimer);
  clearTimeout(l.pollTimer);
  const g = ++l.generation;
  if (!id && !op) return true;
  const r = id
    ? await (operator && p.traffic ? api.stop : api.cancel)(
        p.profile,
        p.activation,
        id,
      )
    : await api.cancelOperation(p.profile, p.activation, p.device.id, op);
  if (!currentRead(c, g)) return false;
  if (!validRead(p, r) || r.code !== 'OK') {
    c.send({ type: 'falhou', error: ppReadMessage(r.code) });
    return false;
  }
  l.activeJob = '';
  l.operation = '';
  if (!operator) l.privateJob = '';
  return true;
}
export async function cancelRead(c: ReadContext) {
  c.life.current.operatorStopped = true;
  c.send({ type: 'metadata', values: { cancelling: true } });
  try {
    if (await releaseRead(c, true)) c.send({ type: 'parado' });
  } catch {
    c.send({
      type: 'falhou',
      error:
        'Cancelamento não confirmado; o prazo da leitura continua limitado.',
    });
  } finally {
    c.send({ type: 'metadata', values: { cancelling: false } });
  }
}
export async function pageRead(c: ReadContext, cursor: string) {
  const p = c.latest.current,
    g = c.life.current.generation;
  c.send({ type: 'metadata', values: { pageBusy: true } });
  try {
    const r = await api.page(
      p.profile,
      p.activation,
      c.state.current.resultJob,
      cursor,
    );
    if (!currentRead(c, g)) return;
    if (!validRead(p, r) || r.code !== 'OK' || !r.data?.read) {
      c.send({ type: 'falhou', error: ppReadMessage(r.code) });
      return;
    }
    c.send({
      type: 'amostra',
      result: r.data.read,
      next: r.data.next_cursor || '',
    });
    c.send({ type: 'metadata', values: { pageCursor: cursor } });
  } catch {
    if (currentRead(c, g))
      c.send({ type: 'falhou', error: 'Página da leitura indisponível.' });
  } finally {
    if (currentRead(c, g))
      c.send({ type: 'metadata', values: { pageBusy: false } });
  }
}
