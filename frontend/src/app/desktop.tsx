import { useCallback, useContext, useState } from 'react';
import type { DesktopProps, Page } from '../features/access/desktop-model';
import { normalizeRoute } from '../features/access/desktop-model';
import { CompanyList, Settings } from './desktop-pages';
import { Sidebar, Topbar } from './desktop-shell';
import type { SessionRouteTarget } from '../features/equipment/session-navigation';
import { SessionRoute } from '../features/equipment/session-navigation';
import { Button } from '../shared/ui/Button';
import { DetailTitle } from '../shared/ui/DetailTitle';
import { DialogHost } from '../shared/ui/dialog-host';
export { Logo } from './desktop-controls';
export type {
  Branding,
  Company,
  DesktopProps,
  Device,
  Operations,
  Page,
  RouteName,
} from '../features/access/desktop-model';
export { Button } from '../shared/ui/Button';
export { Dialog } from '../shared/ui/Dialog';

export function Statusbar({
  connected,
}: {
  connected: boolean;
  access?: boolean;
  network?: string;
  lastQuery?: string;
  toggleTheme?: () => void;
  theme?: string;
}) {
  return connected ? (
    <span className="sr-only" role="status">
      Serviço ativo
    </span>
  ) : (
    <p role="status" className="note">
      Reconectando ao serviço
    </p>
  );
}
export function Desktop(
  p: DesktopProps & { onRoot?: (root: HTMLDivElement | null) => void },
) {
  const externalSession = useContext(SessionRoute);
  const [localSession, setLocalSession] = useState<SessionRouteTarget>();
  const session = externalSession || {
    active: !!localSession,
    target: localSession,
    open: setLocalSession,
    back: () => setLocalSession(undefined),
    leave: () => setLocalSession(undefined),
  };
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const onRoot = p.onRoot;
  const attachRoot = useCallback(
    (node: HTMLDivElement | null) => {
      setHost(node);
      onRoot?.(node);
    },
    [onRoot],
  );
  const route = session.active ? 'sessao' : normalizeRoute(p.page);
  const [detailTitle, setDetailTitle] = useState('');
  const [expandRequest, setExpandRequest] = useState(0);
  const navigationProps = {
    ...p,
    navigate: (page: Page) => {
      const next = normalizeRoute(page);
      if (next === 'equipment' || next === 'credentials')
        setExpandRequest((n) => n + 1);
      p.navigate(page);
    },
  };
  return (
    <SessionRoute.Provider value={session}>
      <DialogHost value={host}>
        <DetailTitle value={setDetailTitle}>
          <div
            ref={attachRoot}
            className="app"
            data-route={route}
            data-theme={p.theme}
          >
            {route === 'companies' ? (
              <div className="col">
                <main>
                  <CompanyList p={p} />
                </main>
                <div className="tema" role="group" aria-label="Tema">
                  <Button
                    aria-pressed={p.theme !== 'dark'}
                    onClick={() => p.setTheme('light')}
                  >
                    Claro
                  </Button>
                  <Button
                    aria-pressed={p.theme === 'dark'}
                    onClick={() => p.setTheme('dark')}
                  >
                    Escuro
                  </Button>
                </div>
              </div>
            ) : (
              <>
                <Sidebar
                  key={route}
                  p={navigationProps}
                  route={route}
                  expandRequest={expandRequest}
                  detailTitle={detailTitle}
                />
                <div className="col">
                  <Topbar
                    p={navigationProps}
                    route={route}
                    detailTitle={detailTitle}
                  />
                  <main>
                    {route === 'overview' ? (
                      <h1>Visão geral</h1>
                    ) : route === 'settings' ? (
                      <Settings p={p} />
                    ) : (
                      p.inventory
                    )}
                    {p.error && <p role="alert">{p.error}</p>}
                    <Statusbar connected={p.connected} />
                  </main>
                </div>
              </>
            )}
            {p.children}
          </div>
        </DetailTitle>
      </DialogHost>
    </SessionRoute.Provider>
  );
}
