import { createContext, useContext, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { ReactNode } from 'react';
import type { DesktopProps, Page, RouteName } from './desktop-model';
import {
  emptyOperations,
  normalizeRoute,
  routeLegacyPageNumber,
} from './desktop-model';
import {
  DesktopPageContent,
  DesktopSidebar,
  DesktopTopbar,
} from './desktop-shell';
import { Icon } from './icons';

import { Button } from './desktop-controls';

export { Button, Logo } from './desktop-controls';
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
  network,
  lastQuery,
  access = false,
  toggleTheme,
  theme,
}: {
  connected: boolean;
  network?: string;
  lastQuery?: string;
  access?: boolean;
  toggleTheme?: () => void;
  theme?: string;
}) {
  return (
    <footer
      className="statusbar"
      role="status"
      aria-label={connected ? 'Serviço ativo' : 'Reconectando ao serviço'}
    >
      <span>
        <span className={'status-dot ' + (!connected ? 'offline' : '')} />
        {connected ? 'Serviço ativo' : 'Reconectando ao serviço'}
      </span>
      <span>
        {access ? 'Conta local · 0.6.0' : network || 'Inventário por empresa'}
      </span>
      <span>
        {lastQuery
          ? 'Última consulta ' + lastQuery
          : access
            ? ''
            : 'Consultas sob demanda · Equipamentos'}
        {toggleTheme && (
          <button
            className="status-theme"
            aria-label={'Tema ' + (theme === 'dark' ? 'escuro' : 'claro')}
            onClick={toggleTheme}
          >
            Tema {theme === 'dark' ? 'escuro' : 'claro'}
          </button>
        )}
      </span>
    </footer>
  );
}

const DialogHost = createContext<HTMLElement | null>(null);
const FOCUSABLE =
  'input:not(:disabled):not([type="hidden"]),button:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex="0"]';

function trapDialogKey(
  e: React.KeyboardEvent,
  ref: React.RefObject<HTMLElement | null>,
  close: () => void,
) {
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
    close();
    return;
  }
  if (e.key !== 'Tab') return;
  const f = Array.from(
    ref.current?.querySelectorAll<HTMLElement>(FOCUSABLE) || [],
  ).filter((el) => !el.closest('[hidden],[aria-hidden="true"]'));
  if (!f.length) {
    e.preventDefault();
    return;
  }
  const active = ref.current?.ownerDocument.activeElement;
  if (!f.some((el) => el === active)) {
    e.preventDefault();
    (e.shiftKey ? f.at(-1) : f[0])?.focus();
  } else if (e.shiftKey && active === f[0]) {
    e.preventDefault();
    f.at(-1)?.focus();
  } else if (!e.shiftKey && active === f.at(-1)) {
    e.preventDefault();
    f[0].focus();
  }
}

export function Dialog({
  title,
  children,
  close,
  portal = false,
  dismissOnBackdrop = true,
  inactive = false,
}: {
  title: string;
  children: ReactNode;
  close: () => void;
  portal?: boolean;
  dismissOnBackdrop?: boolean;
  inactive?: boolean;
}) {
  const host = useContext(DialogHost);
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    const el = ref.current;
    const before = el?.ownerDocument.activeElement as HTMLElement | null;
    const f = Array.from(
      el?.querySelectorAll<HTMLElement>(FOCUSABLE) || [],
    ).filter((node) => !node.closest('[hidden],[aria-hidden="true"]'));
    f[0]?.focus();
    return () => {
      if (before?.isConnected) before.focus();
    };
  }, []);
  const content = (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (dismissOnBackdrop && !inactive && e.target === e.currentTarget)
          close();
      }}
      onKeyDown={(e) => !inactive && trapDialogKey(e, ref, close)}
    >
      <section
        ref={ref}
        className="dialog"
        role="dialog"
        inert={inactive}
        aria-hidden={inactive || undefined}
        aria-modal="true"
        aria-label={title}
      >
        <div className="row between">
          <h2>{title}</h2>
          <Button
            kind="flat dialog-close"
            onClick={close}
            label="Fechar painel"
          >
            <Icon name="XMark" />
          </Button>
        </div>
        {children}
      </section>
    </div>
  );
  return portal && host ? createPortal(content, host) : content;
}

export function Desktop(p: DesktopProps) {
  const [dialogHost, setDialogHost] = useState<HTMLDivElement | null>(null);
  const ops = p.operations || emptyOperations;
  const route = normalizeRoute(p.page);
  const legacyPage = routeLegacyPageNumber(route);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const navTrigger = useRef<HTMLButtonElement>(null);
  const navigation = useRef<HTMLElement>(null);
  useEffect(() => {
    if (navOpen)
      navigation.current
        ?.querySelector<HTMLButtonElement>('.company-picker')
        ?.focus();
  }, [navOpen]);
  const navigate = (next: RouteName) => {
    p.navigate(
      typeof p.page === 'number' ? (routeLegacyPageNumber(next) as Page) : next,
    );
    if (navOpen) {
      setNavOpen(false);
      navTrigger.current?.focus();
    }
  };
  const closeNavigation = () => {
    setNavOpen(false);
    navTrigger.current?.focus();
  };
  const add = p.addEquipment || (() => navigate('equipment'));
  return (
    <DialogHost value={dialogHost}>
      <div
        ref={setDialogHost}
        className="app"
        data-route={route}
        data-page={legacyPage}
        data-theme={p.theme}
        data-accent={p.brand.accent}
      >
        <div className="client">
          <div
            className={'desktop-layout ' + (navOpen ? 'navigation-open' : '')}
            onKeyDown={(e) => {
              if (navOpen && e.key === 'Escape') {
                e.stopPropagation();
                closeNavigation();
              }
            }}
          >
            <DesktopSidebar
              p={p}
              route={route}
              navigation={navigation}
              closeNavigation={closeNavigation}
              navigate={navigate}
              settingsOpen={settingsOpen}
              setSettingsOpen={setSettingsOpen}
            />
            <main className="main">
              <DesktopTopbar
                p={p}
                route={route}
                ops={ops}
                add={add}
                navOpen={navOpen}
                setNavOpen={setNavOpen}
                navTrigger={navTrigger}
                closeNavigation={closeNavigation}
                navigate={navigate}
                setSettingsOpen={setSettingsOpen}
              />
              <DesktopPageContent p={p} route={route} ops={ops} add={add} />
            </main>
          </div>
          <Statusbar
            connected={p.connected}
            network={ops.network}
            lastQuery={ops.lastQuery}
          />
        </div>
        {p.children}
      </div>
    </DialogHost>
  );
}
