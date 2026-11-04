import { useEffect, useState } from 'react';
import { equipmentAPI } from './equipment-api';
// Stored inventory only, including inactive devices; a partial page must never become a usage count.
export function useCredentialUsage(profile: string, activation: string) {
  const key = [profile, activation].join('|');
  const [state, setState] = useState<{
    key: string;
    counts: Record<string, number>;
  }>();
  useEffect(() => {
    let alive = true;
    void (async () => {
      const counts: Record<string, number> = {},
        seen = new Set<string>();
      const cursors = new Set<number>();
      let cursor = 0;
      do {
        if (cursors.has(cursor)) return;
        cursors.add(cursor);
        const result = await equipmentAPI.list(
          profile,
          activation,
          '',
          cursor,
          true,
        );
        if (
          !alive ||
          result.code !== 'OK' ||
          result.profile_id !== profile ||
          result.activation_id !== activation ||
          !result.data
        )
          return;
        for (const device of result.data.devices) {
          if (seen.has(device.id)) continue;
          seen.add(device.id);
          if (device.credential_id)
            counts[device.credential_id] =
              (counts[device.credential_id] || 0) + 1;
        }
        cursor = result.data.next_cursor;
        if (
          !cursor &&
          result.data.total_devices !== undefined &&
          seen.size !== result.data.total_devices
        )
          return;
      } while (cursor);
      if (alive) setState({ key, counts });
    })().catch(() => {
      /* Missing usage is shown explicitly; no count is inferred from a failed page. */
    });
    return () => {
      alive = false;
    };
  }, [profile, activation, key]);
  return state?.key === key ? state.counts : undefined;
}
