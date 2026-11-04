import { DetailTitle } from './ui/DetailTitle';
import { Button } from './ui/Button';
import { CompanyList, Settings } from './desktop-pages';
import { Sidebar, Topbar } from './desktop-shell';
import { useState } from 'react';
import type { DesktopProps } from './desktop-model';
import { normalizeRoute } from './desktop-model';
import { DialogHost } from './ui/Dialog';
export { Button } from './ui/Button';
export { Dialog } from './ui/Dialog';
export { Logo } from './desktop-controls';
export { Appearance } from './appearance';
export type {
  Page,
  RouteName,
  Branding,
  Company,
  Device,
  Operations,
  DesktopProps,
} from './desktop-model';

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
export function Desktop(p: DesktopProps) {
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const route = normalizeRoute(p.page);
  const [detailTitle, setDetailTitle] = useState('');
  return (
    <DialogHost value={host}>
      <DetailTitle value={setDetailTitle}>
        <div
          ref={setHost}
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
              <Sidebar key={route} p={p} route={route} />
              <div className="col">
                <Topbar p={p} route={route} detailTitle={detailTitle} />
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
  );
}
