import type { ReadContext } from './pppoe-read-lifecycle';
import { currentRead, readTarget, validRead } from './pppoe-read-lifecycle';
import { ppReadMessage } from './pppoe-context-actions';
import type { PPResult } from './presentation';
import { RadioCodes } from './radio-contract';
import { pppoeReadAPI as api } from './presentation';
const rejected = [
  'BUSY',
  'REVISION_CONFLICT',
  'INVALID_ARGUMENT',
  'PROFILE_CHANGED',
  'UNAUTHENTICATED',
  'DEVICE_INACTIVE',
  'TRUST_REQUIRED',
  'VAULT_UNAVAILABLE',
  'OBSERVATION_UNAVAILABLE',
  'OBSERVATION_INCOMPATIBLE',
  'LIMIT_EXCEEDED',
  RadioCodes.UNSUPPORTED,
  'DEADLINE_EXCEEDED',
  'SERVICE_UNAVAILABLE',
];
function prepare(c: ReadContext, mode: string, preserve: boolean) {
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
    l.retries = 0;
  }
  if (mode === 'traffic') l.window = undefined;
  l.operatorStopped = false;
  c.send({ type: 'solicitado', scope: mode, preserve });
  return l.generation;
}
export async function startRead(
  c: ReadContext,
  mode: string,
  preserve = false,
) {
  const l = c.life.current,
    p = c.latest.current;
  if (l.operation || l.activeJob || (mode === 'traffic' && l.hidden)) return;
  const g = prepare(c, mode, preserve),
    target = readTarget(p, mode);
  try {
    const r = await api.start(
      p.profile,
      p.activation,
      p.device.id,
      l.operation,
      mode,
      target.obs,
      target.rows,
      p.device.revision,
    );
    await acceptStart(c, { r, g, mode });
  } catch {
    if (currentRead(c, g)) {
      c.send({
        type: 'falhou',
        error:
          'Não foi possível confirmar a solicitação. Cancele a solicitação pendente antes de iniciar outra.',
      });
      if (mode === 'traffic') await c.failed('SERVICE_UNAVAILABLE');
    }
  } finally {
    if (currentRead(c, g))
      c.send({ type: 'metadata', values: { starting: false } });
  }
}

async function acceptStart(
  c: ReadContext,
  { r, g, mode }: { r: PPResult; g: number; mode: string },
) {
  const l = c.life.current,
    p = c.latest.current;
  if (!currentRead(c, g)) {
    if (validRead(p, r) && r.data?.job)
      void api.cancel(p.profile, p.activation, r.data.job.id).catch(() => {});
    return;
  }
  if (!validRead(p, r) || r.code !== 'OK' || !r.data?.job) {
    if (rejected.includes(r.code)) l.operation = '';
    c.send({
      type: 'falhou',
      error: ppReadMessage(r.code),
      pending: Boolean(l.operation),
    });
    if (validRead(p, r) && mode === 'traffic') await c.failed(r.code);
    return;
  }
  l.operation = '';
  l.activeJob = r.data.job.id;
  if (mode === 'secret' || mode === 'traffic') l.privateJob = r.data.job.id;
  c.send({ type: 'iniciado', job: r.data.job.id });
}
