import { useEffect, useState } from 'react';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import type { Device } from '../equipment/equipment-api';
import type { RadioLink } from '../radio/radio-api';
import { EquipmentChain } from '../equipment/equipment-chain';

export function SessionChain(p: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  open?: (id: string) => void;
}) {
  const key = [p.profile, p.activation, p.device.id, p.observation, p.row].join(
    '|',
  );
  const [state, setState] = useState<{
    key: string;
    link: RadioLink;
    devices: Device[];
  }>();
  useEffect(() => {
    let alive = true;
    void Promise.resolve()
      .then(async () => {
        const r = await radioAPI.link(
          p.profile,
          p.activation,
          p.device.id,
          p.observation,
          p.row,
        );
        if (
          !alive ||
          r.code !== 'OK' ||
          r.profile_id !== p.profile ||
          r.activation_id !== p.activation ||
          !r.data?.link?.cpe_id
        )
          return;
        const link = r.data.link;
        const ids = [link.cpe_id, link.ap_id].filter(Boolean);
        const results = await Promise.all(
          ids.map((id) => equipmentAPI.get(p.profile, p.activation, id)),
        );
        if (
          !alive ||
          results.some(
            (item, index) =>
              item.code !== 'OK' ||
              item.profile_id !== p.profile ||
              item.activation_id !== p.activation ||
              item.data?.device?.id !== ids[index],
          )
        )
          return;
        setState({
          key,
          link,
          devices: results.map((item) => item.data!.device!),
        });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [key, p.profile, p.activation, p.device.id, p.observation, p.row]);
  if (state?.key !== key) return null;
  const cpe = state.devices.find((item) => item.id === state.link.cpe_id);
  if (!cpe) return null;
  return (
    <div className="chain">
      <EquipmentChain
        device={cpe}
        devices={[p.device, ...state.devices]}
        link={state.link}
        open={(id) => p.open?.(id)}
      />
    </div>
  );
}
