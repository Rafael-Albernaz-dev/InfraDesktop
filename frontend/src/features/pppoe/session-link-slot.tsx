import { createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import type { Device } from '../equipment/equipment-api';

// Encaixe da aba Enlace: o PPPoE não conhece o rádio (mesma direção dos módulos Go);
// quem compõe a página preenche o painel.
export type SessionLinkPanelProps = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  pppoeUptime: string;
  blocked: boolean;
  onOpenEquipment?: (deviceID: string) => void;
};
type RenderLink = (props: SessionLinkPanelProps) => ReactNode;
const SessionLinkSlot = createContext<RenderLink | undefined>(undefined);

export function SessionLinkProvider(p: {
  render: RenderLink;
  children: ReactNode;
}) {
  return (
    <SessionLinkSlot.Provider value={p.render}>
      {p.children}
    </SessionLinkSlot.Provider>
  );
}

export function SessionLinkPanel(props: SessionLinkPanelProps) {
  const render = useContext(SessionLinkSlot);
  return render ? render(props) : null;
}
