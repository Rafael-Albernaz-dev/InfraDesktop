import { useEffect, useState } from 'react';
import { radioAPI, equipmentAPI } from './presentation';
import { Button } from './ui/Button';
export function SessionCPE(p: {
  profile: string;
  activation: string;
  deviceId: string;
  observation: string;
  row: string;
  open?: (id: string) => void;
}) {
  const key = [p.profile, p.activation, p.deviceId, p.observation, p.row].join(
    '|',
  );
  const [state, setState] = useState<{
    key: string;
    id?: string;
    name?: string;
    failed?: boolean;
  }>();
  useEffect(() => {
    let alive = true;
    void Promise.resolve()
      .then(async () => {
        const r = await radioAPI.link(
          p.profile,
          p.activation,
          p.deviceId,
          p.observation,
          p.row,
        );
        if (!alive) return;
        if (
          r.code !== 'OK' ||
          r.profile_id !== p.profile ||
          r.activation_id !== p.activation
        )
          throw Error('link unavailable');
        const id = r.data?.link?.cpe_id;
        if (!id) {
          setState({ key });
          return;
        }
        const d = await equipmentAPI.get(p.profile, p.activation, id);
        if (!alive) return;
        if (
          d.code !== 'OK' ||
          d.profile_id !== p.profile ||
          d.activation_id !== p.activation ||
          !d.data?.device
        )
          throw Error('device unavailable');
        setState({ key, id, name: d.data.device.name });
      })
      .catch(() => {
        if (alive) setState({ key, failed: true });
      });
    return () => {
      alive = false;
    };
  }, [key, p.profile, p.activation, p.deviceId, p.observation, p.row]);
  if (state?.key !== key) return <span className="muted">Lendo vínculo…</span>;
  if (state.failed) return <span>Não informado nesta leitura</span>;
  return state.id ? (
    <Button className="row-trigger" onClick={() => p.open?.(state.id!)}>
      {state.name}
    </Button>
  ) : (
    <>Sem vínculo</>
  );
}
