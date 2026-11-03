import { RadioModes } from './radio-contract';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { radioAPI as api } from './radio-api';
import type { RadioInput, RadioResult, RadioView } from './radio-api';
import type { Device } from './equipment-api';
import { radioMessage } from './radio-messages';

type RadioObservationContext = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
};

function useRadioState(context: RadioObservationContext) {
  const [view, setView] = useState<RadioView>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [jobID, setJobID] = useState('');
  const life = useRef({ generation: 0 });
  const operation = useRef('');
  const job = useRef('');
  const stopping = useRef(false);
  const root = useRef<HTMLElement>(null);
  return {
    ...context,
    view,
    setView,
    error,
    setError,
    busy,
    setBusy,
    jobID,
    setJobID,
    life,
    operation,
    job,
    stopping,
    root,
  };
}
type RadioState = ReturnType<typeof useRadioState>;

function observationIdentity(state: RadioState) {
  return [
    state.profile,
    state.activation,
    state.device.id,
    state.device.revision,
    state.observation,
    state.row,
  ].join('|');
}

function useRadioLifecycle(state: RadioState) {
  const { life: lifeRef, operation, profile, activation, device } = state;
  const identity = observationIdentity(state);
  useLayoutEffect(() => {
    const lifecycle = lifeRef.current;
    lifecycle.generation++;
    return () => {
      lifecycle.generation++;
    };
  }, [identity, lifeRef]);
  useEffect(
    () => () => {
      if (operation.current)
        void api
          .cancelOperation(profile, activation, device.id, operation.current)
          .catch(() => {});
    },
    [profile, activation, device.id, operation],
  );
}

function acceptRadioReply(
  state: RadioState,
  result: RadioResult,
  generation: number,
) {
  return (
    generation === state.life.current.generation &&
    result.profile_id === state.profile &&
    result.activation_id === state.activation
  );
}

function createRadioStop(state: RadioState) {
  const { stopping: stoppingRef, operation: operationRef, job: jobRef } = state;
  return async function stop() {
    if (stoppingRef.current) return;
    stoppingRef.current = true;
    const generation = state.life.current.generation;
    try {
      const result = jobRef.current
        ? await api.cancel(state.profile, state.activation, jobRef.current)
        : operationRef.current
          ? await api.cancelOperation(
              state.profile,
              state.activation,
              state.device.id,
              operationRef.current,
            )
          : undefined;
      if (generation !== state.life.current.generation) return;
      if (result && result.code !== 'OK') {
        state.setError(radioMessage(result.code));
        return;
      }
      operationRef.current = '';
      jobRef.current = '';
      state.setJobID('');
      state.setBusy(false);
    } catch {
      if (generation === state.life.current.generation)
        state.setError(
          'Parada incerta. Confira o estado da ação antes de iniciar outra.',
        );
    } finally {
      stoppingRef.current = false;
    }
  };
}

function useRadioVisibility(state: RadioState, stop: () => Promise<void>) {
  const { root, operation } = state;
  const stopRef = useRef(stop);
  useLayoutEffect(() => {
    stopRef.current = stop;
  });
  useEffect(() => {
    const ownedDocument = root.current?.ownerDocument;
    if (!ownedDocument) return;
    const hidden = () => {
      if (ownedDocument.visibilityState !== 'visible' && operation.current)
        void stopRef.current();
    };
    ownedDocument.addEventListener('visibilitychange', hidden);
    const observer =
      typeof IntersectionObserver === 'undefined'
        ? undefined
        : new IntersectionObserver((entries) => {
            if (
              entries.some((entry) => !entry.isIntersecting) &&
              operation.current
            )
              void stopRef.current();
          });
    if (root.current) observer?.observe(root.current);
    return () => {
      ownedDocument.removeEventListener('visibilitychange', hidden);
      observer?.disconnect();
    };
  }, [root, operation]);
}

type RadioPoll = { alive: boolean; timer?: ReturnType<typeof setTimeout> };

function mergeRadioPoll(state: RadioState, result: RadioResult) {
  const { operation: operationRef, job: jobRef } = state;
  if (result.data) {
    state.setView((old) => ({
      total: 0,
      more: false,
      next_offset: 0,
      ...old,
      proposal: undefined,
      ...result.data,
    }));
  }
  if (result.data?.job?.state === 'RUNNING') return true;
  operationRef.current = '';
  jobRef.current = '';
  state.setJobID('');
  state.setBusy(false);
  if (
    result.data?.job?.code &&
    !['OK', 'CANCELLED'].includes(result.data.job.code)
  )
    state.setError(radioMessage(result.data.job.code));
  return false;
}

function createRadioPoll(
  state: RadioState,
  generation: number,
  operation: string,
) {
  const owned: RadioPoll = { alive: true };
  async function poll() {
    try {
      const result = await api.status(
        state.profile,
        state.activation,
        state.jobID,
      );
      if (
        !owned.alive ||
        !acceptRadioReply(state, result, generation) ||
        state.stopping.current ||
        state.operation.current !== operation
      )
        return;
      if (result.code !== 'OK') {
        state.setError(radioMessage(result.code));
        return;
      }
      if (mergeRadioPoll(state, result))
        owned.timer = setTimeout(() => void poll(), 200);
    } catch {
      if (owned.alive && generation === state.life.current.generation)
        state.setError(
          'Não foi possível consultar o estado. Use Parar para encerrar a ação; nenhuma leitura foi reenviada.',
        );
    }
  }
  return {
    poll,
    stop: () => {
      owned.alive = false;
      clearTimeout(owned.timer);
    },
  };
}

function useRadioPolling(state: RadioState) {
  const { busy, jobID, profile, activation } = state;
  const identity = observationIdentity(state);
  useEffect(() => {
    if (!busy || !jobID) return;
    const owned = createRadioPoll(
      state,
      state.life.current.generation,
      state.operation.current,
    );
    void owned.poll();
    return owned.stop;
    // Only a deliberate start creates a polling owner; replies update presentation without renewing it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [busy, jobID, profile, activation, identity]);
}

function radioStartInput(
  state: RadioState,
  mode: string,
  family: string | undefined,
  operation: string,
): RadioInput {
  return {
    device_id: state.device.id,
    revision: state.device.revision,
    operation_id: operation,
    mode,
    ...(family ? { family } : {}),
    ...(state.row
      ? { observation_id: state.observation, row_id: state.row }
      : {}),
  };
}

function createRadioStart(state: RadioState) {
  const {
    operation: operationRef,
    job: jobRef,
    stopping: stoppingRef,
    life: lifeRef,
  } = state;
  return async function start(mode: string, family?: string) {
    if (operationRef.current || stoppingRef.current) return;
    const generation = ++lifeRef.current.generation,
      operation = crypto.randomUUID();
    operationRef.current = operation;
    state.setError('');
    state.setBusy(true);
    if (mode === RadioModes.index)
      state.setView((old) => (old ? { ...old, proposal: undefined } : old));
    try {
      const result = await api.start(
        state.profile,
        state.activation,
        radioStartInput(state, mode, family, operation),
      );
      if (
        !acceptRadioReply(state, result, generation) ||
        stoppingRef.current ||
        operationRef.current !== operation
      ) {
        cancelLateRadioStart(state, operation);
        return;
      }
      if (result.code !== 'OK' || !result.data?.job) {
        operationRef.current = '';
        state.setBusy(false);
        state.setError(radioMessage(result.code));
        return;
      }
      jobRef.current = result.data.job.id;
      state.setJobID(result.data.job.id);
      state.setView((old) => ({
        total: 0,
        more: false,
        next_offset: 0,
        ...old,
        ...result.data,
      }));
    } catch {
      if (
        generation === state.life.current.generation &&
        operationRef.current === operation &&
        !stoppingRef.current
      )
        state.setError(
          'Resposta incerta. Use Parar para encerrar a ação antes de iniciar outra.',
        );
    }
  };
}

export function useRadio(
  profile: string,
  activation: string,
  device: Device,
  observation = '',
  row = '',
) {
  const state = useRadioState({
    profile,
    activation,
    device,
    observation,
    row,
  });
  useRadioLifecycle(state);
  const stop = createRadioStop(state);
  useRadioVisibility(state, stop);
  useRadioPolling(state);
  return {
    view: state.view,
    setView: state.setView,
    error: state.error,
    setError: state.setError,
    busy: state.busy,
    start: createRadioStart(state),
    stop,
    root: state.root,
    owner: () => state.life.current.generation,
  };
}

function cancelLateRadioStart(state: RadioState, operation: string) {
  void api
    .cancelOperation(
      state.profile,
      state.activation,
      state.device.id,
      operation,
    )
    .catch(() => {});
}
