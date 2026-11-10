import {
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import {
  EquipmentAccessDetails,
  EquipmentSecondaryActions,
} from '../../features/equipment/equipment-access';
import type { Device } from '../../features/equipment/equipment-api';
import { EquipmentChain } from '../../features/equipment/equipment-chain';
import { EquipmentInsights } from './equipment-insights';
import { EquipmentReading } from '../../features/equipment/equipment-reading';
import { EquipmentQueryFailure } from '../../features/equipment/equipment-reading-status';
import { driverOwners } from '../../features/equipment/equipment-role-options';
import { EquipmentSheetHeader } from './equipment-sheet-header';
import type { useEquipmentState } from '../../features/equipment/equipment-state';
import { useSummaryRefresh } from './equipment-summary-refresh';
import {
  equipmentQueryRunning,
  equipmentReadBlocked,
} from './equipment-query-policy';
import { SummaryTab } from './equipment-summary-tab';
import { PPPoE } from './pppoe-with-link';
import { DeviceMonitoring } from '../../features/pppoe/device-monitoring';
import { PPPoEReadPanel } from '../../features/pppoe/pppoe-diagnostics';
import { RadioEquipment } from '../../features/radio/radio';
import { RadioRoles } from '../../features/equipment/radio-contract';
import { SessionRoute } from '../../features/equipment/session-navigation';
import { DataBlock } from '../../shared/ui/DataBlock';
import { HeaderActionHost } from '../../shared/ui/HeaderAction';
import { Tabs } from '../../shared/ui/Tabs';
import type { RefreshOpening } from '../../features/equipment/use-refresh-on-open';

export type Props = ReturnType<typeof useEquipmentState> & {
  selected: Device;
  profile: string;
  activation: string;
  connected: boolean;
  locked: boolean;
  onBackReady?: (back?: () => void) => void;
};
const TABS: Record<string, string[]> = {
  concentrator: [
    'Resumo',
    'Sessões',
    'Interfaces e saúde',
    'Serviços',
    'Acesso',
  ],
  [RadioRoles.ap]: ['Resumo', 'Clientes', 'Rádio', 'Interfaces', 'Acesso'],
  [RadioRoles.cpe]: ['Resumo', 'Enlace', 'Cliente', 'Roteador', 'Acesso'],
  [RadioRoles.ptp]: ['Resumo', 'Enlace', 'Acesso'],
  other: ['Resumo', 'Acesso'],
};
function AccessTab({ p }: { p: Props }) {
  return (
    <div className="grid">
      <DataBlock
        title="Acesso"
        source="cadastro local"
        data={{
          state: 'ready',
          content: (
            <>
              <EquipmentAccessDetails {...p} />
              <div className="actions">
                <EquipmentSecondaryActions {...p} />
              </div>
            </>
          ),
        }}
      />
    </div>
  );
}
function HealthTab({ p }: { p: Props }) {
  const [sub, setSub] = useState('Interfaces');
  const blocked = equipmentReadBlocked(p) || !p.selected.fingerprint;
  return (
    <>
      <Tabs
        label="Interfaces e saúde"
        sub
        items={['Interfaces', 'Portas e saúde', 'Monitoramento'].map((s) => [
          s,
          s,
        ])}
        value={sub}
        onChange={setSub}
      />
      {sub === 'Interfaces' ? (
        <>
          {p.selected.result ? (
            <EquipmentReading
              device={p.selected}
              focus={false}
              section="interfaces"
            />
          ) : (
            <DataBlock
              title="Interfaces"
              source="SSH"
              data={{
                state: 'unread',
                action: () => void p.startOperation('query', p.selected),
                disabled: blocked,
              }}
            />
          )}
          <PPPoEReadPanel
            profile={p.profile}
            activation={p.activation}
            device={p.selected}
            observation=""
            rows={[]}
            health
            healthView="uplinks"
            blocked={blocked}
          />
        </>
      ) : sub === 'Monitoramento' ? (
        <DeviceMonitoring
          profile={p.profile}
          activation={p.activation}
          device={p.selected}
        />
      ) : (
        <>
          <EquipmentInsights
            profile={p.profile}
            activation={p.activation}
            device={p.selected}
            blocked={blocked}
          />
        </>
      )}
    </>
  );
}
function SheetContent({ p, tab }: { p: Props; tab: string }) {
  if (tab === 'Acesso') return <AccessTab p={p} />;
  if (tab === 'Resumo' && driverOwners[p.selected.role || ''])
    return (
      <DataBlock
        title="Resumo"
        source="cadastro local"
        data={{
          state: 'unbuilt',
          slice: driverOwners[p.selected.role || ''],
          label: 'Não feito',
        }}
      />
    );
  if (tab === 'Resumo') return <SummaryTab p={p} />;
  if (tab === 'Interfaces e saúde') return <HealthTab p={p} />;
  if (tab === 'Sessões' || tab === 'Serviços')
    return (
      <PPPoE
        key={tab}
        profile={p.profile}
        activation={p.activation}
        device={p.selected}
        section={tab === 'Sessões' ? 'sessions' : 'services'}
        sessionTarget={p.sessionTarget}
        onSessionConsumed={() => p.setSessionTarget(undefined)}
        refreshAvailable={
          p.connected && p.vault && !p.locked && !equipmentQueryRunning(p)
        }
        back={() => p.setDetailOpen(false)}
        openEquipment={p.openById}
      />
    );
  return (
    <DataBlock
      title={tab}
      source="cadastro local"
      data={{
        state: 'unbuilt',
        slice: tab === 'Roteador' ? 'L-ROT' : 'L07-DRV',
      }}
    />
  );
}
export function EquipmentDetailPane(p: Props) {
  const sessionRoute = useContext(SessionRoute);
  const { detailSurfaceRef } = p;
  const [tab, setTab] = useState(
    p.sessionTarget?.deviceID === p.selected.id
      ? 'Sessões'
      : sessionRoute?.returnOrigin === p.selected.id
        ? 'Cliente'
        : 'Resumo',
  );
  useEffect(() => {
    if (sessionRoute?.returnOrigin === p.selected.id)
      sessionRoute.consumeReturnOrigin?.();
  }, [sessionRoute, p.selected.id]);
  const radioRefreshDecision = useRef<RefreshOpening | undefined>(undefined);
  const [radioActions, setRadioActions] = useState<HTMLDivElement | null>(null);
  const [chainHost, setChainHost] = useState<HTMLDivElement | null>(null);
  const device = p.selected,
    role = device.role || 'other';
  const isRadio = [RadioRoles.ap, RadioRoles.cpe, RadioRoles.ptp].some(
    (r) => r === role,
  );
  const noDriver = Boolean(driverOwners[role]);
  useSummaryRefresh(p, tab, isRadio || noDriver);
  const back = createEquipmentBack(p);
  useLayoutEffect(() => {
    p.onBackReady?.(back);
    return () => p.onBackReady?.();
  });
  return (
    <section
      ref={detailSurfaceRef}
      tabIndex={-1}
      className="equipment-sheet"
      aria-label="Ficha do equipamento"
    >
      <div hidden={sessionRoute?.active}>
        <EquipmentSheetHeader
          p={p}
          tab={tab}
          isRadio={isRadio}
          back={back}
          setRadioActions={setRadioActions}
        />
        <div className="chain" ref={setChainHost}>
          {!isRadio && (
            <EquipmentChain
              device={device}
              devices={p.devices}
              open={p.openById}
            />
          )}
        </div>
        <EquipmentQueryFailure state={p} tab={tab} isRadio={isRadio} />
        <Tabs
          label="Ficha do equipamento"
          items={(TABS[role] || TABS.other).map((t) => [t, t])}
          value={tab}
          onChange={setTab}
        />
      </div>
      <HeaderActionHost value={radioActions}>
        {isRadio && (
          <RadioEquipment
            key={[p.profile, p.activation, device.id, device.revision].join(
              '|',
            )}
            profile={p.profile}
            activation={p.activation}
            device={device}
            blocked={p.locked || !p.vault || !device.active}
            refreshAvailable={p.connected && p.vault && !p.locked}
            refreshDecision={radioRefreshDecision}
            tab={tab}
            actionHost={radioActions}
            chainHost={chainHost}
            devices={p.devices}
            openEquipment={p.openById}
            openSession={p.openSession}
          />
        )}
        {(!isRadio || tab === 'Acesso' || tab === 'Roteador') && (
          <SheetContent p={p} tab={tab} />
        )}
      </HeaderActionHost>
    </section>
  );
}

function createEquipmentBack(p: Props) {
  const { surfaceRef, detailReadRef, detailSurfaceRef, selected: device } = p;
  return () => {
    const inventory = detailSurfaceRef.current?.closest(
      '.equipment-capability',
    );
    surfaceRef.current++;
    detailReadRef.current++;
    p.setDetailOpen(false);
    p.setJob(undefined);
    queueMicrotask(() => {
      const row = Array.from(
        inventory?.querySelectorAll<HTMLElement>('[data-device-id]') || [],
      ).find((element) => element.dataset.deviceId === device.id);
      row?.focus({ preventScroll: true });
    });
  };
}
