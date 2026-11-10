import { useCallback } from 'react';
import type { MutableRefObject } from 'react';
import type { Device } from './equipment-api';
import { equipmentAPI, equipmentMessage } from './equipment-api';
import type { LinkedSessionTarget } from './session-navigation';
export function useEquipmentNavigator(p: {
  profile: string;
  activation: string;
  scopeRef: MutableRefObject<string>;
  surfaceRef: MutableRefObject<number>;
  mountedRef: MutableRefObject<boolean>;
  open: (device: Device, target?: LinkedSessionTarget) => void;
  error: (message: string) => void;
}) {
  const { profile, activation, scopeRef, surfaceRef, mountedRef, open, error } =
    p;
  return useCallback(
    (id: string, target?: LinkedSessionTarget) => {
      const intent = ++surfaceRef.current;
      const owns = () =>
        mountedRef.current &&
        scopeRef.current === `${profile}|${activation}` &&
        surfaceRef.current === intent;
      void equipmentAPI
        .get(profile, activation, id)
        .then((result) => {
          if (
            !owns() ||
            result.profile_id !== profile ||
            result.activation_id !== activation
          )
            return;
          if (result.code === 'OK' && result.data?.device?.id === id)
            open(result.data.device, target);
          else
            error(
              equipmentMessage(
                result.code === 'OK' ? 'REVISION_CONFLICT' : result.code,
              ),
            );
        })
        .catch(() => {
          if (owns()) error(equipmentMessage('SERVICE_UNAVAILABLE'));
        });
    },
    [profile, activation, scopeRef, surfaceRef, mountedRef, open, error],
  );
}
