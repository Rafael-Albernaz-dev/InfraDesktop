import type { Device } from '../equipment/equipment-api';
import { pendingPPPoE as pending } from '../access/equipment-receipts';
import type { PPResult } from '../../platform/wails/presentation';
import { pppoeDiagnosticAPI as api } from '../../platform/wails/presentation';
import type { Scope, Source } from './pppoe-diag-state';
import { diagnosticMessage } from './pppoe-diagnostic-message';

export const REJECTED_CODES = [
  'JOB_INTERRUPTED',
  'REVISION_CONFLICT',
  'DEVICE_INACTIVE',
  'TRUST_REQUIRED',
  'OBSERVATION_INCOMPATIBLE',
  'OBSERVATION_UNAVAILABLE',
  'PING_READING_STALE',
  'PING_CONTEXT_UNAVAILABLE',
  'PING_SESSION_CHANGED',
  'PING_OFFLINE',
  'BUSY',
  'LIMIT_EXCEEDED',
];

export interface DiagStateForActions {
  job: string;
  setJob: (j: string) => void;
  setError: (e: string) => void;
  setStarting: (s: boolean) => void;
  setUncertain: (u: boolean) => void;
  reload: React.Dispatch<React.SetStateAction<number>>;
}

export function useDiagActions(p: {
  s: DiagStateForActions;
  key: string;
  profile: string;
  activation: string;
  device: Device;
  scope: Scope;
  source?: Source;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  active: (generation: number, result?: PPResult) => boolean;
  acknowledge: (op?: string) => void;
}) {
  const {
    s,
    key,
    profile,
    activation,
    device,
    scope,
    source,
    lifeRef,
    active,
    acknowledge,
  } = p;
  async function queryExistingJob(generation: number) {
    const res = await api.current(profile, activation, device.id, scope);
    if (!active(generation, res)) return;
    if (res.code !== 'OK') {
      s.setError(diagnosticMessage(res.code));
      return;
    }
    const latest = res.data?.attempt;
    if (
      latest?.operation_id === pending.get(key)?.operation &&
      latest?.state === 'RUNNING' &&
      latest.job_id
    ) {
      s.setJob(latest.job_id);
    } else {
      pending.delete(key);
      s.setUncertain(false);
      s.setError(
        'Não há trabalho ativo desta ação. Inicie uma nova leitura deliberadamente.',
      );
      s.reload((n) => n + 1);
    }
  }
  async function start() {
    const generation = lifeRef.current.generation;
    s.setStarting(true);
    s.setError('');
    if (pending.has(key)) {
      try {
        await queryExistingJob(generation);
      } catch {
        if (active(generation))
          s.setError(
            'Não foi possível consultar o estado. Nenhuma sondagem foi reenviada.',
          );
      } finally {
        if (active(generation)) s.setStarting(false);
      }
      return;
    }
    if (pending.size >= 32) {
      s.setError(
        'Há 32 ações incertas. Reconcilie uma delas no contexto original.',
      );
      s.setStarting(false);
      return;
    }
    const receipt = pending.get(key) || {
      operation: crypto.randomUUID(),
      revision: device.revision,
    };
    pending.set(key, receipt);
    s.setUncertain(false);
    try {
      const result =
        scope === 'ping' && source
          ? await api.ping(
              profile,
              activation,
              device.id,
              source.observation,
              source.row,
              receipt.operation,
              device.revision,
            )
          : await api[scope === 'catalog' ? 'catalog' : 'radius'](
              profile,
              activation,
              device.id,
              receipt.operation,
              device.revision,
            );
      if (!active(generation, result)) return;
      if (result.code === 'OK' && result.data?.job)
        s.setJob(result.data.job.id);
      else {
        if (REJECTED_CODES.includes(result.code))
          acknowledge(receipt.operation);
        s.setError(diagnosticMessage(result.code));
        s.setUncertain(pending.has(key));
      }
    } catch {
      if (active(generation)) {
        s.setUncertain(true);
        s.setError(
          'Resposta incerta. Consulte a mesma ação antes de iniciar outra.',
        );
      }
    } finally {
      if (active(generation)) s.setStarting(false);
    }
  }
  async function cancel() {
    const generation = lifeRef.current.generation;
    try {
      const result = await api.cancel(profile, activation, s.job);
      if (active(generation, result) && result.code !== 'OK')
        s.setError(diagnosticMessage(result.code));
    } catch {
      if (active(generation))
        s.setError('Cancelamento incerto. Consulte o estado da ação.');
    }
  }
  return { start, cancel };
}
