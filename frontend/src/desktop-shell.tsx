import { useEffect, useRef, useState } from 'react';
import { Appearance } from './appearance';
import { Button, Logo } from './desktop-controls';
import type { DesktopProps, Operations, RouteName } from './desktop-model';
import { routeBackTarget, routeBreadcrumbs, routeTitle } from './desktop-model';
import {
  Companies,
  Dashboard,
  PPPoERouteView,
  TopologyRouteView,
} from './desktop-overview';
import { Brand, Icon } from './icons';

function ToolbarActions({
  p,
  route,
  ops,
  add,
}: {
  p: DesktopProps;
  route: RouteName;
  ops: Operations;
  add: () => void;
}) {
  if (route === 'companies')
    return (
      <Button
        kind="primary"
        disabled={p.busy || !p.connected}
        onClick={p.create}
      >
        Nova empresa
      </Button>
    );
  if (route === 'overview' && ops.devices.length)
    return (
      <Button kind="flat" onClick={() => p.navigate('equipment')}>
        Equipamentos
      </Button>
    );
  if (
    route === 'equipment' &&
    !p.inventoryDetailOpen &&
    (p.inventory ? p.inventoryHasDevices : ops.devices.length)
  )
    return (
      <Button kind="primary" onClick={add}>
        Adicionar
      </Button>
    );
  return null;
}

function SideNavList(props: {
  route: RouteName;
  navigate: (next: RouteName) => void;
}) {
  const { route, navigate } = props;
  const navItems: Array<[string, string, RouteName | null]> = [
    ['Squares2X2', 'Dashboard', 'overview'],
    ['ServerStack', 'Equipamentos', 'equipment'],
    ['CircleStack', 'Sessões PPPoE', 'pppoe'],
    ['Share', 'Topologia', 'topology'],
    ['Bell', 'Alertas', null],
  ];
  return (
    <nav className="side-nav" aria-label="Navegação principal">
      {navItems.map(([icon, name, target]) => {
        const active = target === route;
        return (
          <button
            key={name}
            className={'nav-item ' + (active ? 'active' : '')}
            aria-current={target === route ? 'page' : undefined}
            disabled={!target}
            title={!target ? `${name}: em breve nesta versão` : undefined}
            onClick={() => target && navigate(target)}
          >
            <Icon name={icon} />
            <span>{name}</span>
            {!target && <span className="count">Em breve</span>}
          </button>
        );
      })}
      <div className="nav-secondary">
        {(['Clock|Histórico', 'Sparkles|Assistência'] as const).map((item) => {
          const [icon, name] = item.split('|');
          return (
            <button
              key={name}
              className="nav-item"
              disabled
              title={`${name}: em breve nesta versão`}
            >
              <Icon name={icon} />
              <span>{name}</span>
              <span className="count">Em breve</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}

export function DesktopSidebar(props: {
  p: DesktopProps;
  route: RouteName;
  navigation: React.RefObject<HTMLElement | null>;
  closeNavigation: () => void;
  navigate: (next: RouteName) => void;
  settingsOpen: boolean;
  setSettingsOpen: (v: boolean) => void;
}) {
  const {
    p,
    route,
    navigation,
    closeNavigation,
    navigate,
    settingsOpen,
    setSettingsOpen,
  } = props;
  return (
    <aside
      className="sidebar"
      ref={navigation}
      id="main-navigation"
      onKeyDown={(e) => e.key === 'Escape' && closeNavigation()}
    >
      <div className="product-brand">
        <Brand />
      </div>
      <button
        className="btn flat nav-close"
        aria-label="Fechar navegação"
        onClick={closeNavigation}
      >
        ×
      </button>
      <button
        className="company-picker"
        onClick={() => navigate('companies')}
        aria-label="Selecionar empresa"
        aria-describedby="active-company-name"
      >
        <Logo brand={p.brand} />
        <span id="active-company-name" className="company-label">
          {p.brand.name || 'Selecionar empresa'}
        </span>
        <span className="caret">⌄</span>
      </button>
      <SideNavList route={route} navigate={navigate} />
      <div className="side-bottom">
        <button
          className={'nav-item ' + (route === 'settings' ? 'active' : '')}
          aria-label="Configurações"
          aria-expanded={settingsOpen}
          aria-controls="settings-navigation"
          onClick={() => setSettingsOpen(!settingsOpen)}
        >
          <Icon name="Cog6Tooth" />
          Configurações
          <span className="caret" aria-hidden="true">
            {settingsOpen ? '⌃' : '⌄'}
          </span>
        </button>
        <div
          id="settings-navigation"
          hidden={!settingsOpen}
          className="settings-navigation"
        >
          <button
            className={'nav-item ' + (route === 'settings' ? 'active' : '')}
            aria-current={route === 'settings' ? 'page' : undefined}
            onClick={() => navigate('settings')}
          >
            Aparência
          </button>
          <button className="nav-item" onClick={() => p.security?.()}>
            Segurança
          </button>
        </div>
      </div>
    </aside>
  );
}

function TopbarBreadcrumbs(props: {
  route: RouteName;
  companyName: string;
  navigate: (next: RouteName) => void;
}) {
  const { route, companyName, navigate } = props;
  const crumbs = routeBreadcrumbs(route, companyName);
  const backTarget = routeBackTarget(route);
  return (
    <div className="topbar-title">
      <span>{routeTitle(route)}</span>
      <nav className="topbar-context" aria-label="Caminho da tela">
        {backTarget && (
          <Button
            kind="flat tiny"
            onClick={() => navigate(backTarget)}
            label="Voltar à tela anterior"
          >
            ← Voltar
          </Button>
        )}
        {crumbs.map((c, idx) => (
          <span key={idx}>
            {idx > 0 ? ' › ' : ''}
            {c.route ? (
              <button
                type="button"
                className="btn flat tiny"
                onClick={() => navigate(c.route!)}
              >
                {c.label}
              </button>
            ) : (
              <strong>{c.label}</strong>
            )}
          </span>
        ))}
      </nav>
    </div>
  );
}

function AccountMenuPanel(props: {
  p: DesktopProps;
  closeAccount: () => void;
  panelRef: React.RefObject<HTMLDivElement | null>;
}) {
  const { p, closeAccount, panelRef } = props;
  return (
    <div id="account-menu" ref={panelRef} className="account-panel">
      <div className="account-identity">
        <strong>{p.account}</strong>
        <span>Conta local</span>
      </div>
      <div className="account-themes" role="group" aria-label="Tema">
        {(['light', 'dark'] as const).map((id) => (
          <button
            key={id}
            aria-pressed={p.theme === id}
            disabled={p.busy}
            onClick={() => p.setTheme(id)}
          >
            {id === 'light' ? 'Claro' : 'Escuro'}
          </button>
        ))}
      </div>
      <button
        onClick={() => {
          closeAccount();
          p.security?.();
        }}
      >
        Segurança
      </button>
      <button
        onClick={() => {
          closeAccount();
          p.logout();
        }}
      >
        Sair
      </button>
      <span className="app-version" aria-label="InfraDesktop versão 0.6.0">
        InfraDesktop 0.6.0
      </span>
    </div>
  );
}

function AccountDropdown(props: {
  p: DesktopProps;
  navigate: (next: RouteName) => void;
  setSettingsOpen: (v: boolean) => void;
}) {
  const { p, navigate, setSettingsOpen } = props;
  const [accountOpen, setAccountOpen] = useState(false);
  const accountTrigger = useRef<HTMLButtonElement>(null);
  const accountPanel = useRef<HTMLDivElement>(null);
  const closeAccount = () => {
    setAccountOpen(false);
    accountTrigger.current?.focus();
  };
  useEffect(() => {
    if (!accountOpen) return;
    const doc = accountPanel.current?.ownerDocument;
    const click = (event: MouseEvent) => {
      if (
        !accountPanel.current?.contains(event.target as Node) &&
        !accountTrigger.current?.contains(event.target as Node)
      )
        setAccountOpen(false);
    };
    doc?.addEventListener('mousedown', click);
    accountPanel.current?.querySelector<HTMLButtonElement>('button')?.focus();
    return () => doc?.removeEventListener('mousedown', click);
  }, [accountOpen]);
  return (
    <div
      className="global-actions"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          closeAccount();
        }
      }}
    >
      <Button
        kind="flat"
        label="Abrir configurações"
        onClick={() => {
          setSettingsOpen(true);
          navigate('settings');
        }}
      >
        <Icon name="Cog6Tooth" />
      </Button>
      <button
        ref={accountTrigger}
        className="btn flat account-trigger"
        aria-label={'Conta: ' + p.account}
        aria-expanded={accountOpen}
        aria-controls="account-menu"
        onClick={() => setAccountOpen(!accountOpen)}
      >
        <span>{p.account}</span>
        <span className="caret" aria-hidden="true">
          {accountOpen ? '⌃' : '⌄'}
        </span>
      </button>
      {accountOpen && (
        <AccountMenuPanel
          p={p}
          closeAccount={closeAccount}
          panelRef={accountPanel}
        />
      )}
    </div>
  );
}

export function DesktopTopbar(props: {
  p: DesktopProps;
  route: RouteName;
  ops: Operations;
  add: () => void;
  navOpen: boolean;
  setNavOpen: (v: boolean) => void;
  navTrigger: React.RefObject<HTMLButtonElement | null>;
  closeNavigation: () => void;
  navigate: (next: RouteName) => void;
  setSettingsOpen: (v: boolean) => void;
}) {
  const {
    p,
    route,
    ops,
    add,
    navOpen,
    setNavOpen,
    navTrigger,
    closeNavigation,
    navigate,
    setSettingsOpen,
  } = props;
  const companyName =
    p.companies.find((c) => c.id === p.activeProfile)?.name ||
    p.brand.name ||
    'Empresa';
  return (
    <header className="topbar">
      <button
        className="btn flat nav-toggle"
        ref={navTrigger}
        aria-label="Navegação"
        aria-expanded={navOpen}
        aria-controls="main-navigation"
        onKeyDown={(e) => e.key === 'Escape' && closeNavigation()}
        onClick={() => setNavOpen(!navOpen)}
      >
        ☰
      </button>
      <TopbarBreadcrumbs
        route={route}
        companyName={companyName}
        navigate={navigate}
      />
      <div className="toolbar-group page-actions">
        <ToolbarActions p={p} route={route} ops={ops} add={add} />
      </div>
      <AccountDropdown
        p={p}
        navigate={navigate}
        setSettingsOpen={setSettingsOpen}
      />
    </header>
  );
}

export function DesktopPageContent(props: {
  p: DesktopProps;
  route: RouteName;
  ops: Operations;
  add: () => void;
}) {
  const { p, route, ops, add } = props;
  return (
    <div className="page-body">
      {p.brandError && route === 'settings' && (
        <div>
          <p role="alert" className="inline-error">
            {p.brandError}
          </p>
          <Button onClick={p.retryBrand} disabled={p.busy}>
            Tentar novamente
          </Button>
        </div>
      )}
      {p.error && (
        <p className="inline-error operation-error" role="alert">
          {p.error}
        </p>
      )}
      {route === 'companies' ? (
        <Companies {...p} />
      ) : route === 'overview' ? (
        <Dashboard
          ops={ops}
          openEquipment={p.openEquipment}
          brand={p.brand}
          inventoryAvailable={Boolean(p.inventory)}
          navigate={p.navigate}
          add={add}
        />
      ) : route === 'equipment' ? (
        p.inventory || (
          <Dashboard
            ops={ops}
            openEquipment={p.openEquipment}
            brand={p.brand}
            inventoryAvailable={Boolean(p.inventory)}
            navigate={p.navigate}
            add={add}
          />
        )
      ) : route === 'pppoe' ? (
        <PPPoERouteView
          ops={ops}
          navigate={p.navigate}
          openEquipment={p.openEquipment}
        />
      ) : route === 'topology' ? (
        <TopologyRouteView
          ops={ops}
          navigate={p.navigate}
          openEquipment={p.openEquipment}
        />
      ) : (
        <Appearance
          key={[
            p.activeProfile,
            p.savedBrand?.name,
            p.savedBrand?.logo,
            p.savedBrand?.accent,
          ].join('|')}
          brand={p.savedBrand || p.brand}
          companySelected={Boolean(p.activeProfile)}
          ready={Boolean(p.saveBrand)}
          catalogName={p.companies.find((c) => c.id === p.activeProfile)?.name}
          theme={p.theme}
          setTheme={p.setTheme}
          busy={p.busy || !p.connected}
          total={ops.total}
          saveBrand={p.saveBrand}
        />
      )}
    </div>
  );
}
