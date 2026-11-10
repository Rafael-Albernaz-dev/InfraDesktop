import { Fragment } from 'react';
import type { Device } from './equipment-api';
import type { RadioLink } from '../radio/radio-api';
import { RadioRoles } from './radio-contract';
import { equipmentType } from './equipment-layout';
export function EquipmentChain({
  device,
  devices,
  link,
  open,
}: {
  device: Device;
  devices: Device[];
  link?: RadioLink;
  open: (id: string) => void;
}) {
  const own = (
    <span className="elo cur" aria-current="page">
      ● {equipmentType(device.role)}: {device.name}
    </span>
  );
  const peer = (label: string, id?: string) =>
    id ? (
      <a
        className="elo"
        href={'#ficha/' + encodeURIComponent(id)}
        onClick={(e) => {
          e.preventDefault();
          open(id);
        }}
      >
        {label}:{' '}
        {devices.find((d) => d.id === id)?.name || 'Equipamento vinculado'}
      </a>
    ) : (
      <span className="elo off">{label}: Sem vínculo</span>
    );
  const items =
    device.role === RadioRoles.cpe
      ? [
          peer('Concentrador', link?.concentrator_id),
          peer('AP', link?.ap_id),
          own,
          <span className="elo off">Roteador: Ainda não existe (L-ROT)</span>,
        ]
      : device.role === RadioRoles.ap
        ? [peer('Concentrador', link?.concentrator_id), own]
        : [own];
  return (
    <>
      <span>Cadeia:</span>
      {items.map((item, i) => (
        <Fragment key={i}>
          {i > 0 && ' › '}
          {item}
        </Fragment>
      ))}
    </>
  );
}
