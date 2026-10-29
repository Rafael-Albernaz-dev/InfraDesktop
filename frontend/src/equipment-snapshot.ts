import { useEffect, useState } from 'react';
import { deviceMonitoringAPI } from './presentation';
import type { PPReadRow } from './presentation';

export function useDisplayNow() {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30000);
    return () => clearInterval(timer);
  }, []);
  return now;
}

// Only an authorized local projection. This hook never starts a remote reading.
export function useEquipmentSnapshot(
  profile: string,
  activation: string,
  device: string,
  revision: number,
  refresh: string | number = 0,
) {
  const key = [profile, activation, device, revision, refresh].join('|');
  const [state, setState] = useState<{
    key: string;
    rows: PPReadRow[];
    failed: boolean;
  }>();
  useEffect(() => {
    let alive = true;
    void Promise.resolve()
      .then(async () => {
        const rows: PPReadRow[] = [];
        let cursor = '',
          pages = 0;
        do {
          const r = await deviceMonitoringAPI.current(
            profile,
            activation,
            device,
            'equipment',
            cursor,
          );
          if (!alive) return;
          if (
            r.profile_id !== profile ||
            r.activation_id !== activation ||
            r.code !== 'OK'
          )
            throw new Error('snapshot unavailable');
          rows.push(...(r.data?.read?.rows || []));
          cursor = r.data?.next_cursor || '';
          pages++;
        } while (cursor && pages < 9);
        if (cursor) throw new Error('snapshot over physical port limit');
        setState({ key, rows, failed: false });
      })
      .catch(() => {
        if (alive) setState({ key, rows: [], failed: true });
      });
    return () => {
      alive = false;
    };
  }, [profile, activation, device, revision, refresh, key]);
  return state?.key === key ? state : undefined;
}
