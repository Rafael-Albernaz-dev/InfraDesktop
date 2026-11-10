import { prepareRead, readTarget } from './pppoe-read-request';
import type { ReadContext } from './pppoe-read-lifecycle';
import { currentRead, validRead } from './pppoe-read-lifecycle';
import { ppReadMessage } from './pppoe-context-actions';
import type { PPResult } from '../../platform/wails/presentation';
import { RadioCodes } from '../equipment/radio-contract';
import { pppoeReadAPI as api } from '../../platform/wails/presentation';
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
export async function startRead(
  c: ReadContext,
  mode: string,
  preserve = false,
) {
  const l = c.life.current,
    p = c.latest.current;
  if (l.operation || l.activeJob || (mode === 'traffic' && l.hidden)) return;
  const g = prepareRead(c, mode, preserve),
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
  l.jobGeneration = g;
  if (mode === 'secret' || mode === 'traffic') l.privateJob = r.data.job.id;
  c.send({ type: 'iniciado', job: r.data.job.id });
}
