import { useEffect, useEffectEvent, useRef } from 'react';
import type { usePPPoEState } from './pppoe-state';
import { pppoeAPI } from './presentation';
export type LinkedSessionTarget = {
  deviceID: string;
  observation: string;
  row: string;
};
export function useOpenLinkedSession(
  s: ReturnType<typeof usePPPoEState>,
  target?: LinkedSessionTarget,
) {
  const consumed = useRef('');
  const open = useEffectEvent(() => {
    if (!target) return;
    if (s.current?.observation_id !== target.observation) {
      s.setError(
        'A leitura da sessão mudou. Reabra o vínculo na ficha da CPE.',
      );
      return;
    }
    void s.runDetail(
      () =>
        pppoeAPI.detail(
          s.profile,
          s.activation,
          target.deviceID,
          target.observation,
          target.row,
        ),
      (r) => {
        s.setPair(undefined);
        s.setDetailTab('traffic');
        s.select(r.data?.rows?.[0]);
      },
    );
  });
  const { context, current, loading } = s;
  const key = target
    ? [context, target.deviceID, target.observation, target.row].join('|')
    : '';
  useEffect(() => {
    if (!key || !current || loading || consumed.current === key) return;
    consumed.current = key;
    open();
  }, [key, current, loading]);
}
