import type { PerformFn } from '../features/access/access-session';
import {
  createBrandSaver,
  useAccessSession,
  useBrandState,
} from '../features/access/access-session';
import { AuthenticatedOverlays } from './app-session';
import type { Page } from './desktop';
import { Desktop } from './desktop';
import { normalizeRoute, type RouteName } from './desktop-model';
import { Equipment } from '../features/equipment/equipment';
import { useEquipmentOperations } from '../features/equipment/equipment-operations';
import type { useInventoryNavigation } from '../features/inventory/inventory-navigation';
import type { NavigationAttempt } from './navigation-guard';
import { NavigationGuard } from './navigation-guard';
import { access } from '../platform/wails/presentation';

function renderInventory(
  brandKey: string,
  profileId: string,
  activationId: string,
  connected: boolean,
  route: RouteName,
  setPage: (page: Page) => void,
  inv: ReturnType<typeof useInventoryNavigation>,
) {
  return (
    <Equipment
      key={brandKey}
      profile={profileId}
      activation={activationId}
      connected={connected}

      primaryInHeader
      credentialsPage={route === 'credentials'}
      addPage={route === 'adicionar'}
      onAdd={() => setPage('adicionar')}
      onAddBack={() => setPage('equipment')}
      targetID={
        inv.inventoryTarget.key === inv.contextPair
          ? inv.inventoryTarget.id
          : undefined
      }
      onSelect={inv.selectInventory}
      onDetailChange={inv.setInventoryDetail}
      onBackReady={inv.registerDetailBack}
      onBlockedChange={inv.setInventoryBlocked}
      onTotal={inv.reportInventory}
    />
  );
}
type AuthenticatedShellProps = {
  theme: string;
  route: RouteName;
  setPage: (p: Page) => void;
  session: ReturnType<typeof useAccessSession>;
  inv: ReturnType<typeof useInventoryNavigation>;
  brand: ReturnType<typeof useBrandState>;
  operations: ReturnType<typeof useEquipmentOperations>;
  perform: PerformFn;
  registerNavigationGuard: (attempt?: NavigationAttempt) => void;
  attemptNavigation: NavigationAttempt;
  root: HTMLDivElement | null;
  setRoot: (root: HTMLDivElement | null) => void;
};
export function AuthenticatedShell(props: AuthenticatedShellProps) {
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
    <NavigationGuard.Provider value={props.registerNavigationGuard}>
      <Desktop
        onRoot={props.setRoot}
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
          route,
          setPage,
          inv,
        )}
        addEquipment={() => {
          setPage('adicionar');
        }}
        page={route}
        navigate={(next: Page) => {
          if (session.securityLockedRef.current) return;
          const navigate = () => {
            session.invalidate();
            if (normalizeRoute(next) === 'equipment') {
              inv.detailBack.current?.();
              inv.selectInventory('');
            }
            setPage(normalizeRoute(next));
            close();
          };
          props.attemptNavigation(navigate);
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
        changePassword={(current, next) =>
          perform(
            () => access.changePassword(current, next),
            undefined,
            true,
            'password',
          )
        }
        passwordError={route === 'settings' && !dialog ? error : ''}
        passwordErrorKind={session.errorKind}
        logout={() => open('Sair desta instalação?')}
        unavailable={(name) => open(name + ' indisponível')}
        error={dialog || route === 'settings' ? '' : error}
      >
        <AuthenticatedOverlays
          session={session}
          perform={perform}
          close={close}
          setPage={setPage}
        />
      </Desktop>
    </NavigationGuard.Provider>
  );
}
