import { useCallback, useEffect, useRef, useState } from 'react';
import type { NavigationAttempt } from './navigation-guard';
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
import { SessionRoute } from './session-navigation';
import type { SessionRouteTarget } from './session-navigation';

export default function App() {
  const [theme, setTheme] = useState('light');
  const [page, setPage] = useState<Page>('companies');
  const [root, setRoot] = useState<HTMLDivElement | null>(null);
  const [sessionTarget, setSessionTarget] = useState<{
    scope: string; target: SessionRouteTarget;
  }>();
  const [returnOrigin, setReturnOrigin] = useState('');
  const navigationGuardRef = useRef<NavigationAttempt | undefined>(undefined);
  const registerNavigationGuard = useCallback((attempt?: NavigationAttempt) => { navigationGuardRef.current = attempt; }, []);
  const attemptNavigation = useCallback((action: () => void) => {
    if (navigationGuardRef.current) navigationGuardRef.current(action);
    else if (!root?.querySelector('[role="dialog"]')) action();
  }, [root]);
  const route = normalizeRoute(page);
  const session = useAccessSession(setPage, setTheme);
  const inv = useInventoryNavigation(
    session.view.profile_id,
    session.view.activation_id,
  );
  const selectedSession = sessionTarget?.scope === inv.contextPair
    ? sessionTarget.target : undefined;
  const sessionActive = route === 'sessao' && !!selectedSession;
  const navigationBack = useNavigationHistory({
    root,
    scope: inv.contextPair,
    authenticated: session.view.authenticated,
    step: {
      route,
      device:
        (route === 'equipment' || sessionActive) &&
        inv.inventoryDetail &&
        inv.inventoryTarget.key === inv.contextPair
          ? inv.inventoryTarget.id
          : '',
      session: sessionActive ? selectedSession : undefined,
    },
    attempt: attemptNavigation,
    blocked:
      session.busy || session.securityLockedRef.current || inv.inventoryBlocked,
    restore: (step) => {
      setReturnOrigin(sessionActive && selectedSession?.originDevice === step.device ? step.device : '');
      if (inv.inventoryDetail && step.device !== inv.inventoryTarget.id)
        inv.detailBack.current?.();
      inv.selectInventory(step.device);
      setSessionTarget(step.session ? { scope: inv.contextPair, target: step.session } : undefined);
      setPage(step.route);
    },
  });
  useEffect(() => {
    const win = root?.ownerDocument.defaultView;
    if (!win) return;
    if (sessionActive) {
      win.history.replaceState(null, '', '#sessao/' + encodeURIComponent(selectedSession.login));
    } else if (win.location.hash.startsWith('#sessao/')) {
      win.history.replaceState(null, '', '#equipamentos');
    }
  }, [root, sessionActive, selectedSession]);
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
    <SessionRoute.Provider value={{
      active: sessionActive,
      target: sessionActive ? selectedSession : undefined,
      returnOrigin,
      consumeReturnOrigin: () => setReturnOrigin(''),
      open: (target) => {
        setReturnOrigin('');
        setSessionTarget({ scope: inv.contextPair, target });
        setPage('sessao');
      },
      back: navigationBack,
      leave: () => setPage('equipment'),
    }}>
    <AuthenticatedShell
      registerNavigationGuard={registerNavigationGuard}
      attemptNavigation={attemptNavigation}
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
    </SessionRoute.Provider>
  );
}
