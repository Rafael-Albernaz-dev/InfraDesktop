import { useEffect } from 'react';
import type { SharedCtx } from './pppoe-state';
import type { PPResult } from './presentation';
import { pppoeAPI as api } from './presentation';
import { equipmentMessage } from './equipment-api';
import { pppoeErrorMessage } from './pppoe-context';
function formatAttemptStatus(r: PPResult): string {
  const a = r.data?.attempt;
  if (
    !a ||
    (a.quality?.status === 'complete' &&
      a.state !== 'FAILED' &&
      a.state !== 'CANCELLED')
  ) {
    return '';
  }
  const kind = a.quality?.status === 'limited' ? 'limitada' : 'incompleta';
  const eligible = a.quality?.counts_known
    ? `${a.quality.eligible} elegíveis; `
    : '';
  const suffix = r.data?.current
    ? 'A tabela continua sendo a última leitura boa, na data indicada.'
    : 'Nenhuma leitura boa disponível.';
  return `Tentativa ${kind}: ${eligible}${equipmentMessage(a.code || 'SERVICE_UNAVAILABLE')} ${suffix}`;
}
export function usePPPoEInitialLoad(p: SharedCtx) {
  const { context, refresh, profile, activation, device, valid, lifeRef } = p;
  useEffect(() => {
    const generation = lifeRef.current.generation;
    let stopped = false;
    void (async () => {
      p.setLoading(true);
      p.setReferencesState('loading');
      p.setInitialFailed(false);
      const r = await api.current(profile, activation, device.id);
      if (stopped || !valid(generation, r)) return;
      if (r.code !== 'OK' || !r.data) {
        p.setError(pppoeErrorMessage(r));
        p.setInitialFailed(true);
        p.setReferencesState('error');
        p.setLoading(false);
        return;
      }
      p.setCurrent((old) =>
        old?.observation_id === r.data?.current?.observation_id
          ? old
          : r.data?.current,
      );
      p.setLastAttempt(r.data.attempt);
      p.setAttempt(formatAttemptStatus(r));
      const rr = await api.references(profile, activation, device.id, '');
      if (stopped || !valid(generation, rr)) return;
      if (rr.code === 'OK') {
        p.setSavedReferences(rr.data?.references || []);
        p.setReferencesState('ready');
      } else {
        p.setError(pppoeErrorMessage(rr));
        p.setReferencesState('error');
      }
      p.setLoading(false);
    })().catch(() => {
      if (!stopped) {
        p.setError('Serviço indisponível. Tente abrir novamente.');
        p.setInitialFailed(true);
        p.setReferencesState('error');
        p.setLoading(false);
      }
    });
    return () => {
      stopped = true;
    };
  }, [context, refresh, profile, activation, device.id, valid, lifeRef]);
}
