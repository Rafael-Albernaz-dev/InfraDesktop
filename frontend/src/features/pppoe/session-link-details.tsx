import { useEffect, useState } from 'react';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import type { RadioReading } from '../radio/radio-api';
import type { RadioLinkState } from '../radio/radio-link-state';

export function useSessionLinkDetails(state: RadioLinkState, enabled: boolean) {
  const ap = state.link?.ap_id || '',
    cpe = state.link?.cpe_id || '';
  const key = [state.profile, state.activation, ap, cpe].join('|');
  const [saved, setSaved] = useState<{
    key: string;
    names: Record<string, string>;
    reading?: RadioReading;
  }>();
  useEffect(() => {
    let alive = true;
    if (!enabled || (!ap && !cpe)) return;
    const ids = [ap, cpe].filter(Boolean);
    void Promise.all([
      Promise.all(
        ids.map((id) =>
          Promise.resolve()
            .then(() => equipmentAPI.get(state.profile, state.activation, id))
            .catch(() => undefined),
        ),
      ),
      cpe
        ? Promise.resolve()
            .then(() => radioAPI.current(state.profile, state.activation, cpe))
            .catch(() => undefined)
        : Promise.resolve(undefined),
    ]).then(([devices, response]) => {
      if (!alive) return;
      const names: Record<string, string> = {};
      ids.forEach((id, index) => {
        const result = devices[index];
        if (
          result?.code === 'OK' &&
          result.profile_id === state.profile &&
          result.activation_id === state.activation &&
          result.data?.device?.id === id
        )
          names[id] = result.data.device.name;
      });
      const reading =
        response?.code === 'OK' &&
        response.profile_id === state.profile &&
        response.activation_id === state.activation
          ? response.data?.reading
          : undefined;
      setSaved({
        key,
        names,
        reading:
          reading?.device_id === cpe && reading.compatible
            ? reading
            : undefined,
      });
    });
    return () => {
      alive = false;
    };
  }, [key, state.profile, state.activation, ap, cpe, enabled]);
  const current = state.radio.view?.cpe_reading;
  const value = saved?.key === key ? saved : { key, names: {} };
  return saved?.key === key && current?.device_id === cpe && current.compatible
    ? { ...value, reading: current }
    : value;
}
