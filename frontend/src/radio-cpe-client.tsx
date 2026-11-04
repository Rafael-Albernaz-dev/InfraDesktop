import { createEquipmentPage } from './radio-equipment-state';
import { useEffect, useState } from 'react';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { LinkedSessionTarget } from './linked-session-navigation';
import type { PPRow } from './presentation';
import { pppoeAPI, equipmentAPI } from './presentation';
import { FRESH_QUERY } from './pppoe-context';
import { equipmentMessage } from './equipment-api';
import { readingAge } from './equipment-reading-format';
import { DataBlock } from './ui/DataBlock';
import type { DataState } from './ui/DataBlock';
import { KeyValues } from './ui/KeyValues';

type LinkedSessionView = {
  key: string;
  row?: PPRow;
  observation?: string;
  at?: string;
  name?: string;
  error?: string;
};
type LinkedSessionContext = {
  key: string;
  profile: string;
  activation: string;
  concentrator: string;
  login: string;
};
function acceptLocalReply(
  r: { code: string; profile_id: string; activation_id: string },
  context: LinkedSessionContext,
) {
  if (
    r.profile_id !== context.profile ||
    r.activation_id !== context.activation
  )
    throw Error('PROFILE_CHANGED');
  if (r.code !== 'OK') throw Error(r.code);
}
async function loadLinkedSession(
  context: LinkedSessionContext,
): Promise<LinkedSessionView> {
  const { key, profile, activation, concentrator } = context;
  const [current, equipment] = await Promise.all([
    pppoeAPI.current(profile, activation, concentrator),
    equipmentAPI.get(profile, activation, concentrator),
  ]);
  acceptLocalReply(current, context);
  acceptLocalReply(equipment, context);
  if (!equipment.data?.device) throw Error('OBSERVATION_UNAVAILABLE');
  const name = equipment.data.device.name,
    observation = current.data?.current;
  if (!observation) return { key, name };
  const matches = await sessionsForLogin(context, observation.observation_id);

  return {
    key,
    row: matches[0],
    observation: observation.observation_id,
    at: observation.provenance.ended_at,
    name,
  };
}
async function sessionsForLogin(
  context: LinkedSessionContext,
  observation: string,
) {
  const { profile, activation, concentrator, login } = context;
  const matches: PPRow[] = [],
    seen = new Set<string>();
  let cursor = '';
  do {
    if (seen.has(cursor)) throw Error('CURSOR_INVALID');
    seen.add(cursor);
    const result = await pppoeAPI.list(
      profile,
      activation,
      concentrator,
      observation,
      { ...FRESH_QUERY(), search: login },
      cursor,
    );
    acceptLocalReply(result, context);
    matches.push(
      ...(result.data?.rows || []).filter((row) => row.login === login),
    );
    if (matches.length > 1)
      throw Error('Vínculo ambíguo; confira as Sessões do concentrador.');
    cursor = result.data?.next_cursor || '';
  } while (cursor);
  return matches;
}
function useLinkedSession(state: RadioEquipmentState) {
  const link = state.radio.view?.link;
  const [refresh, retry] = useState(0);
  const profile = state.profile,
    activation = state.activation,
    concentrator = link?.concentrator_id,
    login = link?.login;
  const key = [
    profile,
    activation,
    state.device.id,
    concentrator,
    login,
    refresh,
  ].join('|');
  const [view, setView] = useState<LinkedSessionView>();
  useEffect(() => {
    if (!concentrator || !login) return;
    let alive = true;
    void loadLinkedSession({ key, profile, activation, concentrator, login })
      .then((next) => {
        if (alive) setView(next);
      })
      .catch((error: unknown) => {
        if (alive)
          setView({
            key,
            error:
              error instanceof Error ? error.message : 'SERVICE_UNAVAILABLE',
          });
      });
    return () => {
      alive = false;
    };
  }, [key, profile, activation, concentrator, login]);
  return {
    link,
    view: view?.key === key ? view : undefined,
    retry: () => retry((n) => n + 1),
  };
}

function LinkedSessionValues({
  state,
  target,
  row,
  name,
}: {
  state: RadioEquipmentState;
  target: LinkedSessionTarget;
  row: PPRow;
  name?: string;
}) {
  return (
    <KeyValues
      rows={[
        [
          'Usuário',
          <a
            href="#"
            onClick={(e) => {
              e.preventDefault();
              state.openSession?.(target);
            }}
          >
            {row.login}
          </a>,
        ],
        ['IP', row.address || 'Não informado nesta leitura'],
        [
          'Concentrador',
          <a
            href={`#ficha/${target.deviceID}`}
            onClick={(e) => {
              e.preventDefault();
              state.openEquipment?.(target.deviceID);
            }}
          >
            {name}
          </a>,
        ],
      ]}
    />
  );
}
function localSessionData(
  state: RadioEquipmentState,
  projection: ReturnType<typeof useLinkedSession>,
): DataState {
  const { link, view, retry } = projection;
  if (state.loading) return { state: 'loading' };
  if (!link) return { state: 'empty', message: 'Sem vínculo' };
  if (view?.error)
    return {
      state: 'failed',
      reason: equipmentMessage(view.error),
      technical: view.error,
      retry,
    };
  if (!view) return { state: 'loading' };
  if (!view.observation) return { state: 'unread', action: retry };
  if (!view.row) return { state: 'empty', message: 'Sem vínculo' };
  const target: LinkedSessionTarget = {
    deviceID: link.concentrator_id,
    observation: view.observation,
    row: view.row.row_id,
  };
  return {
    state: 'ready',
    content: (
      <LinkedSessionValues
        state={state}
        target={target}
        row={view.row}
        name={view.name}
      />
    ),
  };
}
export function RadioCPEClient({ state }: { state: RadioEquipmentState }) {
  const projection = useLinkedSession(state);
  const view = projection.view;
  let data = localSessionData(state, projection);
  if (state.radio.error)
    data = {
      state: 'failed',
      reason: state.radio.error,
      technical: state.radio.view?.job?.code || 'Falha ao abrir vínculo local',
      previous: data.state === 'ready' ? data.content : undefined,
      retry: () => void createEquipmentPage(state)(0, true),
    };
  return (
    <DataBlock
      title="Sessão PPPoE vinculada"
      source={view?.name ? `SSH (${view.name})` : 'cadastro local'}
      age={readingAge(view?.at)}
      data={data}
    />
  );
}
