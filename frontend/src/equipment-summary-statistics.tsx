import { useEffect, useState } from 'react';
import type { Device } from './equipment-api';
import type { PPObservation } from './presentation-pppoe-types';
import { pppoeAPI } from './presentation';
import { useEquipmentSnapshot } from './equipment-snapshot';
import { readingAge } from './equipment-reading-format';
import type { DataState } from './ui/DataBlock';
import { DataBlock } from './ui/DataBlock';

export function EquipmentSummaryStatistics(p: {
  profile: string;
  activation: string;
  device: Device;
}) {
  const { profile, activation, device } = p;
  const [refresh, bump] = useState(0);
  const retry = () => bump((n) => n + 1);
  const snapshot = useEquipmentSnapshot(
    profile,
    activation,
    device.id,
    device.revision,
    refresh,
  );
  const key = [profile, activation, device.id, device.revision, refresh].join(
    '|',
  );
  const [current, setCurrent] = useState<{
    key: string;
    observation?: PPObservation;
    failed?: boolean;
  }>();
  useEffect(() => {
    let alive = true;
    void Promise.resolve()
      .then(() => pppoeAPI.current(profile, activation, device.id))
      .then((r) => {
        if (!alive) return;
        if (
          r.profile_id !== profile ||
          r.activation_id !== activation ||
          r.code !== 'OK'
        ) {
          setCurrent({ key, failed: true });
        } else setCurrent({ key, observation: r.data?.current });
      })
      .catch(() => {
        if (alive) setCurrent({ key, failed: true });
      });
    return () => {
      alive = false;
    };
  }, [profile, activation, device.id, key]);
  const observation = current?.key === key ? current.observation : undefined;
  const restarts = snapshot?.rows.find((row) => row.kind === 'restarts');
  const project = (
    loading: boolean,
    failed: boolean,
    present: boolean,
    content: React.ReactNode,
  ): DataState =>
    loading
      ? { state: 'loading' }
      : failed
        ? {
            state: 'failed',
            reason: 'Falhou a leitura salva',
            technical: 'Projeção local não disponível',
            retry,
          }
        : !present
          ? { state: 'unread', action: retry }
          : { state: 'ready', content };
  return (
    <>
      <DataBlock
        title="Sessões online"
        source="SSH"
        age={readingAge(observation?.provenance.ended_at)}
        data={project(
          current?.key !== key,
          Boolean(current?.failed),
          Boolean(observation),
          observation?.online !== undefined ? (
            <>
              <b>{observation.online}</b> sessões
            </>
          ) : (
            <p>Não informado nesta leitura</p>
          ),
        )}
      />
      <DataBlock
        title="Reinícios"
        source="SSH"
        age={readingAge(restarts?.at)}
        data={
          restarts?.used === 0
            ? { state: 'empty', message: 'Nenhum reinício encontrado' }
            : project(
                !snapshot,
                Boolean(snapshot?.failed),
                Boolean(restarts),
                restarts ? (
                  <p>
                    {restarts.used ?? 'Não informado nesta leitura'} reinícios
                    em 7 d; {restarts.size ?? 'Não informado nesta leitura'} em
                    30 d
                  </p>
                ) : null,
              )
        }
      />
    </>
  );
}
