import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Credential, Device } from '../equipment/equipment-api';
import { equipmentAPI, equipmentMessage } from '../equipment/equipment-api';
import { credentialChangeInput } from './radio-credential-input';
import { useLatestRef } from '../../shared/hooks/use-latest-ref';

export type RadioRecoveryProps = {
  profile: string;
  activation: string;
  deviceID: string;
  initialMessage?: string;
  observed?: (device: Device) => void;
  changed: (device: Device) => void;
};

function useRecoveryLife(
  profile: string,
  activation: string,
  deviceID: string,
) {
  const alive = useRef(true),
    recoveryLife = useRef({});
  useLayoutEffect(() => {
    alive.current = true;
    recoveryLife.current = {};
    return () => {
      alive.current = false;
      recoveryLife.current = {};
    };
  }, [profile, activation, deviceID]);
  return { alive, recoveryLife };
}

export function useRecoveryState(props: RadioRecoveryProps) {
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false);
  const [target, setTarget] = useState<Device>();
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [message, setMessage] = useState(
    props.initialMessage ?? 'Credencial recusada',
  );
  const life = useRecoveryLife(props.profile, props.activation, props.deviceID);
  return {
    ...props,
    ...life,
    open,
    setOpen,
    busy,
    setBusy,
    target,
    setTarget,
    credentials,
    setCredentials,
    message,
    setMessage,
  };
}
type RecoveryState = ReturnType<typeof useRecoveryState>;

function recoveryCurrent(state: RecoveryState, generation: object) {
  return state.alive.current && generation === state.recoveryLife.current;
}

async function availableCredentials(profile: string, activation: string) {
  const all: Credential[] = [];
  let cursor = 0;
  do {
    const result = await equipmentAPI.credentials(profile, activation, cursor);
    if (result.code !== 'OK') throw new Error('credentials');
    all.push(...(result.data?.credentials || []));
    cursor = result.data?.next_cursor || 0;
  } while (cursor);
  return all;
}

export function createRecoveryChoice(state: RecoveryState) {
  return async function choose() {
    const generation = state.recoveryLife.current;
    state.setBusy(true);
    try {
      const result = await equipmentAPI.get(
        state.profile,
        state.activation,
        state.deviceID,
      );
      if (result.code !== 'OK' || !result.data?.device)
        throw new Error('device');
      const all = await availableCredentials(state.profile, state.activation);
      if (recoveryCurrent(state, generation)) {
        state.setTarget(result.data.device);
        state.observed?.(result.data.device);
        state.setCredentials(all);
        state.setOpen(true);
      }
    } catch {
      if (recoveryCurrent(state, generation))
        state.setMessage(
          'Não foi possível abrir as credenciais desta empresa.',
        );
    } finally {
      if (recoveryCurrent(state, generation)) state.setBusy(false);
    }
  };
}

export function useRecoveryAutoload(
  state: RecoveryState,
  choose: () => Promise<void>,
) {
  const { profile, activation, deviceID } = state;
  const chooseRef = useLatestRef(choose);
  useEffect(() => {
    let active = true;
    void Promise.resolve().then(() => {
      if (active) void chooseRef.current();
    });
    return () => {
      active = false;
    };
    // Load once per mounted refusal; a failed load keeps a manual retry.
  }, [profile, activation, deviceID, chooseRef]);
}

export function createRecoverySave(state: RecoveryState) {
  return async function save(credential: string) {
    const generation = state.recoveryLife.current,
      target = state.target;
    if (
      !target ||
      !credential ||
      credential === target.credential_id ||
      state.busy
    )
      return;
    state.setBusy(true);
    try {
      const result = await equipmentAPI.save(
        state.profile,
        state.activation,
        credentialChangeInput(target, credential),
      );
      if (!recoveryCurrent(state, generation)) return;
      if (result.code !== 'OK' || !result.data?.device) {
        state.setMessage(equipmentMessage(result.code));
        return;
      }
      state.setTarget(result.data.device);
      state.setMessage('Credencial lembrada para este equipamento.');
      state.setOpen(false);
      state.changed(result.data.device);
    } catch {
      if (recoveryCurrent(state, generation))
        state.setMessage(
          'Alteração incerta. Reabra o equipamento para conferir a credencial.',
        );
    } finally {
      if (recoveryCurrent(state, generation)) state.setBusy(false);
    }
  };
}
