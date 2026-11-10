import type { PPRow } from '../../platform/wails/presentation';
import { useEffect, useEffectEvent, useRef } from 'react';
import type { usePPPoEState } from './pppoe-state';
import { pppoeAPI } from '../../platform/wails/presentation';

import type { LinkedSessionTarget } from '../equipment/session-navigation';
export type { LinkedSessionTarget };
export function useOpenLinkedSession(
  s: ReturnType<typeof usePPPoEState>,
  target?: LinkedSessionTarget,
  onOpen?: (row: PPRow) => void,
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
        const row = r.data?.rows?.[0];
        s.select(row);
        if (row) onOpen?.(row);
      },
    );
  });
  const { context, current, loading } = s;
  const key = target
    ? [context, target.deviceID, target.observation, target.row].join('|')
    : '';
  useEffect(() => {
    if (!key) consumed.current = '';
    if (!key || !current || loading || consumed.current === key) return;
    consumed.current = key;
    open();
  }, [key, current, loading]);
}
