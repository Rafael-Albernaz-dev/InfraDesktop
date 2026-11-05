import { useState } from 'react';
import type { Page } from './desktop';
import { normalizeRoute } from './desktop-model';
import './ui/tokens.css';
import './ui/kit.css';
import { useEquipmentOperations } from './equipment-operations';
import { useNavigationHistory } from './use-navigation-history';
import {
  createPerformAction,
  UnauthenticatedShell,
  useAccessSession,
  useBrandState,
} from './app-session';
import { useInventoryNavigation } from './inventory-navigation';
import { AuthenticatedShell } from './authenticated-shell';

export default function App() {
  const [theme, setTheme] = useState('light');
  const [page, setPage] = useState<Page>('companies');
  const [root, setRoot] = useState<HTMLDivElement | null>(null);
  const route = normalizeRoute(page);
  const session = useAccessSession(setPage, setTheme);
  const inv = useInventoryNavigation(
    session.view.profile_id,
    session.view.activation_id,
  );
  useNavigationHistory({
    root,
    scope: inv.contextPair,
    authenticated: session.view.authenticated,
    step: {
      route,
      device:
        route === 'equipment' &&
        inv.inventoryDetail &&
        inv.inventoryTarget.key === inv.contextPair
          ? inv.inventoryTarget.id
          : '',
    },
    blocked:
      session.busy || session.securityLockedRef.current || inv.inventoryBlocked,
    restore: (step) => {
      if (inv.inventoryDetail) inv.detailBack.current?.();
      inv.selectInventory(step.device);
      setPage(step.route);
    },
  });
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
      root={root}
      setRoot={setRoot}
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
