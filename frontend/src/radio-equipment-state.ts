import { useEffect, useRef, useState } from 'react';
import { radioAPI as api } from './radio-api';
import type { Device } from './equipment-api';
import { equipmentAPI } from './equipment-api';
import { RadioCodes, RadioFamilies } from './radio-contract';
import { radioMessage } from './radio-messages';
import { useRadio } from './radio-observation';
export type RadioEquipmentProps = {
  profile: string;
  activation: string;
  device: Device;
  blocked: boolean;
  devices?: Device[];
  openEquipment?: (id: string) => void;
};
export function useRadioEquipmentState(props: RadioEquipmentProps) {
  const [currentDevice, setCurrentDevice] = useState(props.device);
  const { root: rootRef, ...radio } = useRadio(
    props.profile,
    props.activation,
    currentDevice,
  );
  const refreshedJob = useRef('');
  const [loading, setLoading] = useState(true),
    [offset, setOffset] = useState(0);
  const [warning, setWarning] = useState(15),
    [critical, setCritical] = useState(20);
  return {
    ...props,
    currentDevice,
    setCurrentDevice,
    rootRef,
    radio,
    refreshedJob,
    loading,
    setLoading,
    offset,
    setOffset,
    warning,
    setWarning,
    critical,
    setCritical,
    credentialRefused:
      !radio.busy &&
      (radio.view?.job?.code === RadioCodes.REMOTE_AUTH_FAILED ||
        (radio.view?.job?.state === 'FAILED' &&
          radio.view?.ap_code === RadioCodes.REMOTE_AUTH_FAILED)),
    reading: radio.view?.reading,
    limits: radio.view?.profile,
  };
}
export type RadioEquipmentState = Omit<
  ReturnType<typeof useRadioEquipmentState>,
  'rootRef'
>;

export function useStoredEquipmentRadio(state: RadioEquipmentState) {
  const {
    profile,
    activation,
    device,
    radio,
    setWarning,
    setCritical,
    setLoading,
  } = state;
  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const result = await api.current(profile, activation, device.id);
        if (!alive) return;
        if (result.code !== 'OK') {
          radio.setError(radioMessage(result.code));
          return;
        }
        radio.setView(result.data);
        if (result.data?.profile) {
          setWarning(result.data.profile.warning);
          setCritical(result.data.profile.critical);
        }
      } catch {
        if (alive) radio.setError('Não foi possível abrir a leitura salva.');
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
    // Read stored data only when the account/device context changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile, activation, device.id]);
}

export function useCompletedEquipmentRefresh(state: RadioEquipmentState) {
  const {
    radio,
    profile,
    activation,
    device,
    setCurrentDevice,
    refreshedJob: refreshedJobRef,
    credentialRefused,
  } = state;
  useEffect(() => {
    const completed = radio.view?.job;
    if (
      radio.busy ||
      !completed ||
      completed.state === 'RUNNING' ||
      credentialRefused ||
      refreshedJobRef.current === completed.id
    )
      return;
    refreshedJobRef.current = completed.id;
    let alive = true;
    void equipmentAPI
      .get(profile, activation, device.id)
      .then((result) => {
        if (alive && result.code === 'OK' && result.data?.device) {
          const next = result.data.device;
          setCurrentDevice((current) =>
            next.revision >= current.revision ? next : current,
          );
        }
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [
    radio.busy,
    radio.view?.job,
    credentialRefused,
    profile,
    activation,
    device.id,
    setCurrentDevice,
    refreshedJobRef,
  ]);
}

export function createFamilyConfirmation(state: RadioEquipmentState) {
  return async function confirm() {
    try {
      const result = await api.confirmFamily(
        state.profile,
        state.activation,
        state.device.id,
        state.radio.view?.reading?.family || RadioFamilies.unsupported,
        state.currentDevice.revision,
        state.warning,
        state.critical,
      );
      if (result.code !== 'OK') {
        state.radio.setError(radioMessage(result.code));
        return;
      }
      state.radio.setView(result.data);
      state.radio.setError('');
    } catch {
      state.radio.setError(
        'Confirmação incerta. Reabra os dados salvos antes de repetir.',
      );
    }
  };
}

export function createEquipmentPage(state: RadioEquipmentState) {
  return async function page(next: number, fresh = false) {
    const owner = state.radio.owner();
    try {
      const result = await api.current(
        state.profile,
        state.activation,
        state.device.id,
        fresh ? '' : state.radio.view?.reading?.id || '',
        next,
      );
      if (owner !== state.radio.owner()) return;
      if (result.code !== 'OK') {
        state.radio.setError(radioMessage(result.code));
        return;
      }
      state.radio.setView(result.data);
      state.radio.setError('');
      state.setOffset(next);
    } catch {
      if (owner === state.radio.owner())
        state.radio.setError('Página indisponível. Reabra a leitura salva.');
    }
  };
}
