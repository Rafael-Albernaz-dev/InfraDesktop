import { useContext, useEffect, useRef, useState } from 'react';
import { SessionRoute } from './session-navigation';
import type { DesktopProps, RouteName } from './desktop-model';
import { Button } from './ui/Button';
import { ProductBrand } from './ui/ProductBrand';
import { ShellIcon } from './ui/ShellIcon';
export function Sidebar({
  p,
  route,
  expandRequest,
  detailTitle,
}: {
  p: DesktopProps;
  route: RouteName;
  expandRequest: number;
  detailTitle: string;
}) {
  const [closedAt, setClosedAt] = useState<{
    request: number;
    title: string;
  }>();
  const expanded =
    (route === 'equipment' || route === 'credentials' || route === 'adicionar' || route === 'sessao') &&
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
            className={route === 'equipment' || route === 'adicionar' || route === 'sessao' ? 'on' : ''}
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
export function Topbar({
  p,
  route,
  detailTitle,
}: {
  p: DesktopProps;
  route: RouteName;
  detailTitle?: string;
}) {
  const session = useContext(SessionRoute);
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
  const title =
    route === 'vitrine'
      ? 'Vitrine'
      : route === 'overview'
        ? 'Visão geral'
        : route === 'settings'
          ? 'Configurações'
          : route === 'credentials'
            ? 'Credenciais'
            : route === 'adicionar'
              ? 'Adicionar equipamento'
            : 'Equipamentos';
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
      <nav className="bc" aria-label="Caminho">
        {(route === 'credentials' || route === 'adicionar' || route === 'sessao' ||
          (route === 'equipment' && detailTitle)) && (
          <>
            <a
              href="#equipamentos"
              onClick={(e) => {
                e.preventDefault();
                p.navigate('equipment');
              }}
            >
              Equipamentos
            </a>
            <span aria-hidden="true">›</span>
          </>
        )}
        {route === 'sessao' && detailTitle && (
          <>
            <a href="#equipamentos" onClick={(event) => {
              event.preventDefault(); session?.back();
            }}>{detailTitle}</a>
            <span aria-hidden="true">›</span>
          </>
        )}
        <b aria-current="page">{route === 'sessao' ? session?.target?.login :
          route === 'equipment' && detailTitle ? detailTitle : title}</b>
      </nav>
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
      <div className="wrap">
        <Button
          ref={profileTrigger}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setMenu('profile');
            }
          }}
          className="user"
          aria-label={`Conta: ${p.account}`}
          aria-haspopup="true"
          aria-expanded={menu === 'profile'}
          onClick={() => setMenu(menu === 'profile' ? '' : 'profile')}
        >
          <span className="av">{p.account.slice(0, 1).toUpperCase()}</span>
          <b>{p.account}</b>
          <ShellIcon name="chev" />
        </Button>
        {menu === 'profile' && (
          <div
            className="dd"
            role="menu"
            ref={profileMenu}
            onKeyDown={(event) => {
              const items = Array.from(
                event.currentTarget.querySelectorAll<HTMLElement>(
                  '[role=menuitem]',
                ),
              );
              const index = items.indexOf(event.target as HTMLElement);
              const next =
                event.key === 'ArrowDown'
                  ? (index + 1) % items.length
                  : event.key === 'ArrowUp'
                    ? (index + items.length - 1) % items.length
                    : event.key === 'Home'
                      ? 0
                      : event.key === 'End'
                        ? items.length - 1
                        : -1;
              if (next >= 0) {
                event.preventDefault();
                items[next]?.focus();
              }
            }}
          >
            <Button
              className="mi"
              disabled={p.busy}
              role="menuitem"
              onClick={() => {
                p.setTheme(p.theme === 'dark' ? 'light' : 'dark');
                setMenu('');
              }}
            >
              <ShellIcon name="moon" />
              Tema {p.theme === 'dark' ? 'claro' : 'escuro'}
            </Button>
            <hr />
            <Button
              className="mi sair"
              role="menuitem"
              onClick={() => {
                setMenu('');
                p.logout();
              }}
            >
              <ShellIcon name="out" />
              Sair da conta
            </Button>
          </div>
        )}
      </div>
    </header>
  );
}
