import {
  SummaryTab,
  equipmentReadBlocked,
  useSummaryRefresh,
} from './equipment-summary-tab';
import { EquipmentChain } from './equipment-chain';
import { equipmentType } from './equipment-layout';
import { HeaderActionHost } from './ui/HeaderAction';
import { useRef, useState } from 'react';
import type { RefreshOpening } from './use-refresh-on-open';
import type { useEquipmentState } from './equipment-state';
import type { Device } from './equipment-api';
import { equipmentAPI as api } from './equipment-api';
import {
  EquipmentAccessDetails,
  EquipmentSecondaryActions,
} from './equipment-access';
import { EquipmentReading, latestSavedReading } from './equipment-reading';
import { EquipmentInsights } from './equipment-insights';
import { formatDuration } from './network-format';
import { DeviceMonitoring, PPPoEReadPanel } from './pppoe-diagnostics';
import { PPPoE } from './pppoe';
import { RadioEquipment } from './radio';
import { RadioRoles } from './radio-contract';
import { Button } from './ui/Button';
import { DataBlock } from './ui/DataBlock';
import { PageHeader } from './ui/PageHeader';
import { Tabs } from './ui/Tabs';

export type Props = ReturnType<typeof useEquipmentState> & {
  selected: Device;
  profile: string;
  activation: string;
  connected: boolean;
  locked: boolean;
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
        refreshAvailable={p.connected && p.vault && !p.locked}
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
        slice: tab === 'Roteador' ? 'L-ROT' : 'L-BASE-2',
      }}
    />
  );
}
export function EquipmentDetailPane(p: Props) {
  const { surfaceRef, detailReadRef, detailSurfaceRef } = p;
  const [tab, setTab] = useState('Resumo');
  const radioRefreshDecision = useRef<RefreshOpening | undefined>(undefined);
  const [radioActions, setRadioActions] = useState<HTMLDivElement | null>(null);
  const [chainHost, setChainHost] = useState<HTMLDivElement | null>(null);
  const device = p.selected,
    role = device.role || 'other';
  const isRadio = [RadioRoles.ap, RadioRoles.cpe, RadioRoles.ptp].some(
    (r) => r === role,
  );
  useSummaryRefresh(p, tab, isRadio);
  const saved = latestSavedReading(device);
  const readingAge = saved.at
    ? `há ${formatDuration((p.displayNow - Date.parse(saved.at)) / 1000)}`
    : 'Ainda sem leitura';
  const back = () => {
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
  return (
    <section
      ref={detailSurfaceRef}
      tabIndex={-1}
      className="equipment-sheet"
      aria-label="Ficha do equipamento"
    >
      <PageHeader
        title={device.name}
        meta={
          <>
            {device.name} · {equipmentType(role)} ·{' '}
            {saved.version || 'Não informado nesta leitura'} ·{' '}
            {saved.at ? `atualizado ${readingAge}` : readingAge}
          </>
        }
      >
        <Button onClick={back}>Voltar</Button>
        {isRadio || tab === 'Sessões' || tab === 'Serviços' ? (
          <div ref={setRadioActions} className="actions header-actions" />
        ) : (
          <Button
            kind="primary"
            disabled={
              p.job?.state === 'RUNNING' ? p.locked : equipmentReadBlocked(p)
            }
            onClick={() =>
              p.job?.state === 'RUNNING'
                ? void p.perform(
                    () => api.cancel(p.profile, p.activation, p.job!.id),
                    (r) => p.setJob(r.data?.job),
                    false,
                  )
                : void p.startOperation(
                    device.fingerprint ? 'query' : 'inspect',
                    device,
                  )
            }
          >
            {p.job?.state === 'RUNNING'
              ? 'Parar'
              : device.fingerprint
                ? 'Atualizar'
                : 'Conectar'}
          </Button>
        )}
      </PageHeader>
      <div className="chain" ref={setChainHost}>
        {!isRadio && (
          <EquipmentChain
            device={device}
            devices={p.devices}
            open={p.openById}
          />
        )}
      </div>
      <Tabs
        label="Ficha do equipamento"
        items={(TABS[role] || TABS.other).map((t) => [t, t])}
        value={tab}
        onChange={setTab}
      />
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
          />
        )}
        {(!isRadio ||
          tab === 'Acesso' ||
          tab === 'Cliente' ||
          tab === 'Roteador') && <SheetContent p={p} tab={tab} />}
      </HeaderActionHost>
    </section>
  );
}
