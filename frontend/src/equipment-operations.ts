import { formatDate } from './network-format';
import { useEffect, useState, useMemo, useCallback } from 'react';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Device } from './equipment-api';
import type { Operations } from './desktop-model';
export function useEquipmentOperations(
  profile: string,
  activation: string,
  connected: boolean,
  active: boolean,
): Operations {
  const key = [profile, activation].join('|');
  const [attempt, setAttempt] = useState(0);
  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  const token = useMemo(
    () => ({ key, active, connected, attempt }),
    [key, active, connected, attempt],
  );
  const [view, setView] = useState<{
    key: { key: string } | null;
    value: Operations;
  }>({
    key: null,
    value: {
      devices: [],
      sessions: [],
      alerts: [],
      activity: [],
      inventoryState: 'loading',
    },
  });
  useEffect(() => {
    if (!profile || !connected || !active) return;
    let stopped = false;
    void (async () => {
      const all: Device[] = [];
      for (const inactive of [false, true]) {
        let cursor = 0;
        const seen = new Set<number>();
        do {
          if (seen.has(cursor) || seen.size >= 16)
            throw new Error('LIMIT_EXCEEDED');
          seen.add(cursor);
          const r = await api.list(profile, activation, '', cursor, inactive);
          if (stopped) return;
          if (r.code !== 'OK' || !r.data) throw new Error(r.code);
          all.push(...r.data.devices);
          cursor = r.data.next_cursor;
        } while (cursor);
      }
      if (stopped) return;
      const unique = [...new Map(all.map((d) => [d.id, d])).values()];
      setView({
        key: token,
        value: {
          devices: unique.map((d) => ({
            id: d.id,
            name: d.name,
            ip: d.address,
            type: d.role === 'concentrator' ? 'Concentrador' : 'Equipamento',
            sessions: d.role === 'concentrator' ? d.pppoe?.online : undefined,
            offline: d.pppoe?.offline,
            pppoeAt: d.pppoe?.at,
            os: d.result?.version
              ? `RouterOS ${d.result.version}`
              : 'Não identificado',
            access: 'SSH',
            state: !d.active
              ? 'Inativo'
              : d.last_attempt?.state === 'FAILED'
                ? 'Tentativa recente falhou'
                : d.result
                  ? 'Leitura disponível'
                  : 'Não consultado',
            style: 'muted',
            time: d.result?.observed_at
              ? formatDate(d.result.observed_at)
              : 'Não consultado',
            detail: d.last_attempt ? equipmentMessage(d.last_attempt.code) : '',
          })),
          sessions: [],
          alerts: [],
          activity: [],
          inventoryState: 'ready',
        },
      });
    })().catch((e: unknown) => {
      if (!stopped)
        setView({
          key: token,
          value: {
            devices: [],
            sessions: [],
            alerts: [],
            activity: [],
            inventoryState: 'error',
            inventoryError: equipmentMessage(
              e instanceof Error ? e.message : 'SERVICE_UNAVAILABLE',
            ),
          },
        });
    });
    return () => {
      stopped = true;
    };
  }, [profile, activation, connected, active, token]);
  return view.key === token || (!connected && view.key?.key === key)
    ? { ...view.value, retry }
    : {
        devices: [],
        sessions: [],
        alerts: [],
        activity: [],
        inventoryState: 'loading',
      };
}
