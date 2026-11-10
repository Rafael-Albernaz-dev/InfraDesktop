import { useEffect, useRef, useState } from 'react';
import { DesktopBreadcrumbs } from './desktop-breadcrumbs';
import type { DesktopProps, RouteName } from '../features/access/desktop-model';
import { DesktopProfileMenu } from './desktop-profile-menu';
import { Button } from '../shared/ui/Button';
import { ProductBrand } from '../shared/ui/ProductBrand';
import { ShellIcon } from '../shared/ui/ShellIcon';
type SidebarProps = {
  p: DesktopProps;
  route: RouteName;
  expandRequest: number;
  detailTitle: string;
};
export function Sidebar({
  p,
  route,
  expandRequest,
  detailTitle,
}: SidebarProps) {
  const [closedAt, setClosedAt] = useState<{
    request: number;
    title: string;
  }>();
  const expanded =
    (route === 'equipment' ||
      route === 'credentials' ||
      route === 'adicionar' ||
      route === 'sessao') &&
    (!closedAt ||
      closedAt.request !== expandRequest ||
      closedAt.title !== detailTitle);
  const navigateTo = (event: React.MouseEvent, next: RouteName) => {
    event.preventDefault();
    p.navigate(next);
  };
  const company = p.companies.find((c) => c.id === p.activeProfile);
  return (
    <nav className="side" aria-label="Menu">
      <div className="nome">
        <ProductBrand />
      </div>
      <a
        href="#empresas"
        className="emp"
        onClick={(e) => navigateTo(e, 'companies')}
      >
        {company?.name || 'Selecionar empresa'} ▾
      </a>
      <a
        href="#visao"
        className={route === 'overview' ? 'on' : ''}
        aria-current={route === 'overview' ? 'page' : undefined}
        onClick={(e) => navigateTo(e, 'overview')}
      >
        Visão geral
      </a>
      <a
        href="#equipamentos"
        className={`grp ${expanded ? 'open' : ''}`}
        aria-expanded={expanded}
        onClick={(e) => {
          e.preventDefault();
          if (!expanded) {
            setClosedAt(undefined);
            if (route !== 'equipment' && route !== 'credentials')
              p.navigate('equipment');
          } else setClosedAt({ request: expandRequest, title: detailTitle });
        }}
      >
        Equipamentos
      </a>
      {expanded && (
        <div className="subm">
          <a
            href="#equipamentos"
            className={
              route === 'equipment' ||
              route === 'adicionar' ||
              route === 'sessao'
                ? 'on'
                : ''
            }
            aria-current={route === 'equipment' ? 'page' : undefined}
            onClick={(e) => navigateTo(e, 'equipment')}
          >
            Lista
          </a>
          <a
            href="#credenciais"
            className={route === 'credentials' ? 'on' : ''}
            aria-current={route === 'credentials' ? 'page' : undefined}
            onClick={(e) => navigateTo(e, 'credentials')}
          >
            Credenciais
          </a>
        </div>
      )}
    </nav>
  );
}
type TopbarProps = {
  p: DesktopProps;
  route: RouteName;
  detailTitle?: string;
};
export function Topbar({ p, route, detailTitle }: TopbarProps) {
  const [menu, setMenu] = useState('');
  const profileTrigger = useRef<HTMLButtonElement>(null);
  const profileMenu = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (menu === 'profile')
      profileMenu.current
        ?.querySelector<HTMLElement>('[role=menuitem]')
        ?.focus();
    const doc = profileTrigger.current?.ownerDocument;
    const dismiss = (event: PointerEvent) => {
      if (
        event.target instanceof Node &&
        !profileTrigger.current?.parentElement?.contains(event.target)
      )
        setMenu('');
    };
    doc?.addEventListener('pointerdown', dismiss);
    return () => doc?.removeEventListener('pointerdown', dismiss);
  }, [menu]);
  return (
    <header
      className="top"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          setMenu('');
          profileTrigger.current?.focus();
        }
      }}
    >
      <DesktopBreadcrumbs p={p} route={route} detailTitle={detailTitle} />
      <div className="wrap">
        <Button
          className="ib"
          label="Notificações"
          aria-haspopup="true"
          aria-expanded={menu === 'notifications'}
          onClick={() =>
            setMenu(menu === 'notifications' ? '' : 'notifications')
          }
        >
          <ShellIcon name="bell" />
        </Button>
        {menu === 'notifications' && (
          <div className="dd" role="menu">
            <div className="dh2">Notificações</div>
            <div className="empty">Nenhuma notificação</div>
          </div>
        )}
      </div>
      <Button
        className="ib"
        label="Configurações"
        onClick={() => p.navigate('settings')}
      >
        <ShellIcon name="gear" />
      </Button>
      <DesktopProfileMenu
        p={p}
        menu={menu}
        setMenu={setMenu}
        profileTrigger={profileTrigger}
        profileMenu={profileMenu}
      />
    </header>
  );
}
