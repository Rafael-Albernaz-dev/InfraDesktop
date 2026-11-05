import { useCallback, useState } from 'react';
import { access } from './presentation';
import { Desktop } from './desktop';
import type { Page } from './desktop';
import { normalizeRoute, type RouteName } from './desktop-model';
import './ui/tokens.css';
import './ui/kit.css';
import { Equipment } from './equipment';
import { useEquipmentOperations } from './equipment-operations';
import {
  AppOverlays,
  createBrandSaver,
  createPerformAction,
  UnauthenticatedShell,
  useAccessSession,
  useBrandState,
} from './app-session';
import type { PerformFn } from './app-session';
function useInventoryNavigation(profileId: string, activationId: string) {
  const [inventoryAction, addInventory] = useState(0);
  const [inventoryTarget, setInventoryTarget] = useState({ key: '', id: '' });
  const [inventoryDetail, setInventoryDetail] = useState(false);
  const [inventoryState, setInventoryState] = useState({ key: '', total: 0 });
  const contextPair = [profileId, activationId].join('|');
  const reportInventory = useCallback((key: string, total: number) => {
    setInventoryState((old) =>
      old.key === key && old.total === total ? old : { key, total },
    );
  }, []);
  const selectInventory = useCallback(
    (id: string) => setInventoryTarget({ key: contextPair, id }),
    [contextPair],
  );
  return {
    contextPair,
    inventoryAction,
    addInventory,
    inventoryTarget,
    inventoryDetail,
    setInventoryDetail,
    inventoryState,
    reportInventory,
    selectInventory,
  };
}
function renderInventory(
  brandKey: string,
  profileId: string,
  activationId: string,
  connected: boolean,
  credentialsPage: boolean,
  inv: ReturnType<typeof useInventoryNavigation>,
) {
  return (
    <Equipment
      key={brandKey}
      profile={profileId}
      activation={activationId}
      connected={connected}
      createRequest={inv.inventoryAction}
      primaryInHeader
      credentialsPage={credentialsPage}
      targetID={
        inv.inventoryTarget.key === inv.contextPair
          ? inv.inventoryTarget.id
          : undefined
      }
      onSelect={inv.selectInventory}
      onDetailChange={inv.setInventoryDetail}
      onTotal={inv.reportInventory}
    />
  );
}
function AuthenticatedOverlays(props: {
  session: ReturnType<typeof useAccessSession>;
  perform: PerformFn;
  close: () => void;
  setPage: (p: Page) => void;
}) {
  const { session, perform, close, setPage } = props;
  return (
    <AppOverlays
      dialog={session.dialog}
      close={close}
      recovery={session.recovery}
      busy={session.busy}
      error={session.error}
      recoveryUncertain={session.recoveryUncertain}
      securityLockedRef={session.securityLockedRef}
      workingRef={session.workingRef}
      perform={perform}
      setRecovery={session.setRecovery}
      setRecoveryUncertain={session.setRecoveryUncertain}
      setPage={setPage}
    />
  );
}
function AuthenticatedShell(props: {
  theme: string;
  route: RouteName;
  setPage: (p: Page) => void;
  session: ReturnType<typeof useAccessSession>;
  inv: ReturnType<typeof useInventoryNavigation>;
  brand: ReturnType<typeof useBrandState>;
  operations: ReturnType<typeof useEquipmentOperations>;
  perform: PerformFn;
}) {
  const { theme, route, setPage, session, inv, brand, operations, perform } =
    props;
  const { view, connected, busy, error, dialog } = session;
  const company = (view.companies || []).find((c) => c.id === view.profile_id);
  const open = (title: string) => {
    if (session.securityLockedRef.current) return;
    session.bumpSurface();
    session.setError('');
    session.setDialog(title);
  };
  const close = () => open('');
  const hasDevices =
    inv.inventoryState.key === inv.contextPair && inv.inventoryState.total > 0;
  return (
    <Desktop
      operations={operations}
      inventoryDetailOpen={inv.inventoryDetail}
      openEquipment={(id) => {
        inv.selectInventory(id);
        setPage('equipment');
      }}
      inventoryHasDevices={hasDevices}
      inventory={renderInventory(
        brand.brandKey,
        view.profile_id,
        view.activation_id,
        connected,
        route === 'credentials',
        inv,
      )}
      addEquipment={() => {
        setPage('equipment');
        inv.addInventory((n) => n + 1);
      }}
      page={route}
      navigate={(next: Page) => {
        if (session.securityLockedRef.current) return;
        session.invalidate();
        if (normalizeRoute(next) === 'equipment') inv.selectInventory('');
        inv.addInventory(0);
        setPage(normalizeRoute(next));
        close();
      }}
      brand={{
        ...brand.savedBrand,
        name: brand.savedBrand.name || company?.name || '',
      }}
      savedBrand={brand.savedBrand}
      brandError={brand.brandError}
      retryBrand={() => brand.retryBrand((n) => n + 1)}
      saveBrand={createBrandSaver(view, brand, perform)}
      account={view.name}
      theme={theme}
      setTheme={(next) => void perform(() => access.setTheme(next))}
      connected={connected}
      busy={busy}
      companies={view.companies || []}
      activeProfile={view.profile_id}
      create={() => open('Nova empresa')}
      enter={(id) =>
        void perform(
          () => access.selectCompany(id),
          () => setPage('overview'),
        )
      }
      security={() => open('Segurança')}
      logout={() => open('Sair desta instalação?')}
      unavailable={(name) => open(name + ' indisponível')}
      error={dialog ? '' : error}
    >
      <AuthenticatedOverlays
        session={session}
        perform={perform}
        close={close}
        setPage={setPage}
      />
    </Desktop>
  );
}
export default function App() {
  const [theme, setTheme] = useState('light');
  const [page, setPage] = useState<Page>('companies');
  const route = normalizeRoute(page);
  const session = useAccessSession(setPage, setTheme);
  const inv = useInventoryNavigation(
    session.view.profile_id,
    session.view.activation_id,
  );
  const brand = useBrandState(session.view, session.connected);
  const operations = useEquipmentOperations(
    session.view.profile_id,
    session.view.activation_id,
    session.connected && session.view.authenticated,
    route === 'overview',
  );
  const perform = createPerformAction(session);
  if (!session.view.authenticated) {
    return (
      <UnauthenticatedShell
        theme={theme}
        setTheme={setTheme}
        session={session}
        perform={perform}
      />
    );
  }
  return (
    <AuthenticatedShell
      theme={theme}
      route={route}
      setPage={setPage}
      session={session}
      inv={inv}
      brand={brand}
      operations={operations}
      perform={perform}
    />
  );
}
