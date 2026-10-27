import { Button, Logo } from './desktop-controls';
import { Appearance } from './appearance';
export { Button, Logo } from './desktop-controls';
export { Appearance } from './appearance';
import { createContext, useContext, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { CSSProperties, ReactNode } from 'react';
import { Brand, Icon } from './icons';
import { emptyOperations } from './desktop-model';
import type {
  Page,
  Branding,
  Company,
  Device,
  Operations,
} from './desktop-model';
export type {
  Page,
  Branding,
  Company,
  Device,
  Operations,
} from './desktop-model';
function State({
  children,
  kind = '',
}: {
  children: ReactNode;
  kind?: string;
}) {
  return <span className={'text-state ' + kind}>{children}</span>;
}
function KV({
  label,
  children,
  mono = false,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="kv">
      <span className="key">{label}</span>
      <span className={mono ? 'mono' : undefined}>{children}</span>
    </div>
  );
}
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
  const focusables = () =>
    Array.from(
      ref.current?.querySelectorAll<HTMLElement>(
        'input:not(:disabled):not([type="hidden"]),button:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex="0"]',
      ) || [],
    ).filter((el) => !el.closest('[hidden],[aria-hidden="true"]'));
  useEffect(() => {
    const el = ref.current;
    const before = el?.ownerDocument.activeElement as HTMLElement | null;
    focusables()[0]?.focus();
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
      onKeyDown={(e) => {
        if (inactive) return;
        if (e.key === 'Escape') {
          e.preventDefault();
          e.stopPropagation();
          close();
        }
        if (e.key === 'Tab') {
          const f = focusables();
          if (!f.length) {
            e.preventDefault();
            return;
          }
          const active = ref.current?.ownerDocument.activeElement;
          if (!f.some((el) => el === active)) {
            e.preventDefault();
            (e.shiftKey ? f.at(-1) : f[0])?.focus();
            return;
          }
          if (e.shiftKey && ref.current?.ownerDocument.activeElement === f[0]) {
            e.preventDefault();
            f.at(-1)?.focus();
          } else if (
            !e.shiftKey &&
            ref.current?.ownerDocument.activeElement === f.at(-1)
          ) {
            e.preventDefault();
            f[0].focus();
          }
        }
      }}
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
export type DesktopProps = {
  page: Page;
  navigate: (page: Page) => void;
  brand: Branding;
  account: string;
  theme: string;
  setTheme: (theme: string) => void;
  connected: boolean;
  busy: boolean;
  companies: Company[];
  activeProfile: string;
  create: () => void;
  enter: (id: string) => void;
  logout: () => void;
  security?: () => void;
  unavailable: (title: string) => void;
  operations?: Operations;
  openEquipment?: (id: string) => void;
  inventory?: ReactNode;
  addEquipment?: () => void;
  inventoryHasDevices?: boolean;
  inventoryDetailOpen?: boolean;
  error?: string;
  saveBrand?: (brand: Branding) => void | boolean | Promise<boolean>;
  savedBrand?: Branding;
  brandError?: string;
  retryBrand?: () => void;
  children?: ReactNode;
};
function ToolbarActions({
  p,
  ops,
  add,
}: {
  p: DesktopProps;
  ops: Operations;
  add: () => void;
}) {
  if (p.page === 2)
    return (
      <Button
        kind="primary"
        disabled={p.busy || !p.connected}
        onClick={p.create}
      >
        Nova empresa
      </Button>
    );
  if (p.page === 3 && ops.devices.length)
    return (
      <Button kind="flat" onClick={() => p.navigate(4)}>
        Equipamentos
      </Button>
    );
  if (
    p.page === 4 &&
    !p.inventoryDetailOpen &&
    (p.inventory ? p.inventoryHasDevices : ops.devices.length)
  )
    return (
      <Button kind="primary" onClick={add}>
        Adicionar
      </Button>
    );
  if (p.page === 5 && ops.target)
    return (
      <>
        <Button kind="flat" onClick={() => p.navigate(6)}>
          Topologia
        </Button>
        <Button disabled>Terminal</Button>
      </>
    );
  if (p.page === 6 && ops.devices.length)
    return (
      <select aria-label="Tipo de mapa" defaultValue="Topologia">
        <option>Topologia</option>
        <option disabled>Geográfico</option>
      </select>
    );
  return null;
}
export function Desktop(p: DesktopProps) {
  const [dialogHost, setDialogHost] = useState<HTMLDivElement | null>(null);
  const ops = p.operations || emptyOperations;
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
  const navigate = (page: Page) => {
    p.navigate(page);
    if (navOpen) {
      setNavOpen(false);
      navTrigger.current?.focus();
    }
  };
  const closeNavigation = () => {
    setNavOpen(false);
    navTrigger.current?.focus();
  };
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

  const title =
    p.page === 5 && ops.target ? (
      <>
        <span>{ops.devices.find((d) => d.id === ops.target)?.name}</span>
        <span className="topbar-context">
          {ops.devices.find((d) => d.id === ops.target)?.ip}
        </span>
      </>
    ) : (
      {
        2: 'Empresas',
        3: 'Dashboard',
        4: 'Equipamentos',
        5: 'Concentrador',
        6: 'Topologia',
        7: 'Aparência',
      }[p.page]
    );
  const add = p.addEquipment || (() => p.unavailable('Adicionar equipamento'));
  return (
    <DialogHost value={dialogHost}>
      <div
        ref={setDialogHost}
        className="app"
        data-page={p.page}
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
            <aside
              className="sidebar"
              ref={navigation}
              id="main-navigation"
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  closeNavigation();
                }
              }}
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
                onClick={() => navigate(2)}
                aria-label="Selecionar empresa"
                aria-describedby="active-company-name"
              >
                <Logo brand={p.brand} />
                <span id="active-company-name" className="company-label">
                  {p.brand.name || 'Selecionar empresa'}
                </span>
                <span className="caret">⌄</span>
              </button>
              <nav className="side-nav" aria-label="Navegação principal">
                {(
                  [
                    ['Squares2X2', 'Dashboard', 3],
                    ['ServerStack', 'Equipamentos', 4],
                    ['Share', 'Topologia', 6],
                    ['Bell', 'Alertas', 0],
                  ] as const
                ).map(([icon, name, page]) => (
                  <button
                    key={name}
                    className={
                      'nav-item ' +
                      (page === p.page || (page === 4 && p.page === 5)
                        ? 'active'
                        : '')
                    }
                    aria-current={page === p.page ? 'page' : undefined}
                    onClick={() => {
                      if (page) {
                        navigate(page);
                      } else p.unavailable('Alertas');
                    }}
                  >
                    <Icon name={icon} />
                    <span>{name}</span>
                    {page === 0 && ops.alerts.length > 0 && (
                      <span className="count">{ops.alerts.length}</span>
                    )}
                  </button>
                ))}
                <div className="nav-secondary">
                  {[
                    ['Clock', 'Histórico'],
                    ['Sparkles', 'Assistência'],
                  ].map(([icon, name]) => (
                    <button
                      key={name}
                      className="nav-item"
                      onClick={() => p.unavailable(name)}
                    >
                      <Icon name={icon} />
                      {name}
                    </button>
                  ))}
                </div>
              </nav>
              <div className="side-bottom">
                <button
                  className={'nav-item ' + (p.page === 7 ? 'active' : '')}
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
                    className={'nav-item ' + (p.page === 7 ? 'active' : '')}
                    aria-current={p.page === 7 ? 'page' : undefined}
                    onClick={() => {
                      navigate(7);
                    }}
                  >
                    Aparência
                  </button>
                  <button
                    className="nav-item"
                    onClick={() => {
                      p.security?.();
                    }}
                  >
                    Segurança
                  </button>
                </div>
              </div>
            </aside>
            <main className="main">
              <header className="topbar">
                <button
                  className="btn flat nav-toggle"
                  ref={navTrigger}
                  aria-label="Navegação"
                  aria-expanded={navOpen}
                  aria-controls="main-navigation"
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') closeNavigation();
                  }}
                  onClick={() => setNavOpen(!navOpen)}
                >
                  ☰
                </button>
                <div className="topbar-title">{title}</div>
                <div className="toolbar-group page-actions">
                  <ToolbarActions p={p} ops={ops} add={add} />
                </div>
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
                      p.navigate(7);
                    }}
                  >
                    <Icon name="Cog6Tooth" />
                  </Button>
                  <Button
                    kind="flat"
                    label="Notificações indisponíveis"
                    disabled
                  >
                    <Icon name="Bell" />
                  </Button>
                  <button
                    ref={accountTrigger}
                    className="account-trigger"
                    aria-label={'Conta: ' + p.account}
                    aria-expanded={accountOpen}
                    aria-controls="account-actions"
                    onClick={() => setAccountOpen(!accountOpen)}
                  >
                    <span className="account-initials" aria-hidden="true">
                      {p.account.slice(0, 2).toUpperCase()}
                    </span>
                    <span className="account-name">{p.account}</span>
                    <span aria-hidden="true">⌄</span>
                  </button>
                  {accountOpen && (
                    <div
                      id="account-actions"
                      className="account-dropdown"
                      ref={accountPanel}
                      onBlur={(e) => {
                        if (!e.currentTarget.contains(e.relatedTarget))
                          setAccountOpen(false);
                      }}
                    >
                      <span className="dropdown-label">Tema</span>
                      <div className="theme-menu">
                        {(['light', 'dark'] as const).map((id) => (
                          <button
                            key={id}
                            type="button"
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
                      <span
                        className="app-version"
                        aria-label="InfraDesktop versão 0.6.0"
                      >
                        InfraDesktop 0.6.0
                      </span>
                    </div>
                  )}
                </div>
              </header>
              <div className="page-body">
                {p.brandError && p.page === 7 && (
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
                {p.page === 2 ? (
                  <Companies {...p} />
                ) : p.page === 3 ? (
                  <Dashboard
                    ops={ops}
                    openEquipment={p.openEquipment}
                    brand={p.brand}
                    inventoryAvailable={Boolean(p.inventory)}
                    navigate={p.navigate}
                    add={add}
                  />
                ) : p.page === 4 ? (
                  p.inventory || (
                    <Inventory
                      ops={ops}
                      navigate={p.navigate}
                      unavailable={p.unavailable}
                      add={add}
                    />
                  )
                ) : p.page === 5 ? (
                  ops.inventoryState ? (
                    <div className="empty-state">
                      <h2>Sessões PPPoE</h2>
                      <p>
                        Escolha um concentrador salvo e abra suas sessões no
                        equipamento.
                      </p>
                      <Button onClick={() => p.navigate(4)}>
                        Ver equipamentos
                      </Button>
                    </div>
                  ) : (
                    <Concentrator
                      ops={ops}
                      navigate={p.navigate}
                      unavailable={p.unavailable}
                    />
                  )
                ) : p.page === 6 ? (
                  ops.inventoryState ? (
                    <div className="empty-state">
                      <h2>Topologia ainda indisponível</h2>
                      <p>
                        Os equipamentos cadastrados estão no inventário da
                        empresa.
                      </p>
                      <Button onClick={() => p.navigate(4)}>
                        Ver equipamentos
                      </Button>
                    </div>
                  ) : (
                    <Topology
                      ops={ops}
                      navigate={p.navigate}
                      unavailable={p.unavailable}
                      add={add}
                    />
                  )
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
                    catalogName={
                      p.companies.find((c) => c.id === p.activeProfile)?.name
                    }
                    theme={p.theme}
                    setTheme={p.setTheme}
                    busy={p.busy || !p.connected}
                    total={ops.total}
                    saveBrand={p.saveBrand}
                  />
                )}
              </div>
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
function Empty({
  title = 'Nenhum equipamento',
  children,
}: {
  title?: string;
  children: ReactNode;
}) {
  return (
    <div className="empty-surface">
      <h1>{title}</h1>
      {children}
    </div>
  );
}
function Companies(p: DesktopProps) {
  const [chosen, setChosen] = useState('');
  const selected =
    p.companies.find((c) => c.id === (chosen || p.activeProfile)) ||
    p.companies[0];
  return (
    <>
      <div className="company-intro">
        <h1>Empresas</h1>
        <p>
          {`${p.companies.length} ${p.companies.length === 1 ? 'empresa' : 'empresas'} nesta instalação`}
        </p>
      </div>
      <div className="company-layout">
        <section className="company-list">
          {p.companies.length ? (
            <table className="app-table company-table">
              <thead>
                <tr>
                  <th>Empresa</th>
                  <th>Conexão</th>
                  <th style={{ textAlign: 'right' }}>Equipamentos</th>
                </tr>
              </thead>
              <tbody>
                {p.companies.map((c) => (
                  <tr
                    key={c.id}
                    className={
                      'company-row selectable ' +
                      (selected?.id === c.id ? 'selected' : '')
                    }
                    tabIndex={0}
                    aria-selected={selected?.id === c.id}
                    aria-label={'Selecionar ' + c.name}
                    onClick={() => setChosen(c.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setChosen(c.id);
                      }
                    }}
                  >
                    <td>
                      <span className="cell-name">{c.name}</span>
                      <span className="cell-sub">
                        {c.lastAccess || 'Perfil desta instalação'}
                      </span>
                    </td>
                    <td>
                      <State kind={c.connectionStyle}>
                        {c.connection || 'Não consultada'}
                      </State>
                    </td>
                    <td style={{ textAlign: 'right' }}>{c.count ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <Empty title="Nenhuma empresa cadastrada">
              <Button kind="primary" onClick={p.create} disabled={p.busy}>
                Criar empresa
              </Button>
            </Empty>
          )}
        </section>
        <aside className="company-detail">
          <Logo
            brand={
              selected?.id === p.activeProfile
                ? p.brand
                : { name: selected?.name || '', logo: '', accent: 'teal' }
            }
            size="large"
          />
          <h2>{selected?.name || 'Seu primeiro perfil'}</h2>
          <KV label="Rede">{selected?.network || 'Não consultada'}</KV>
          <KV label="Inventário">
            {selected?.inventory || 'Inventário ainda não consultado'}
          </KV>
          <KV label="Acessos de equipamentos">
            {selected?.access || 'Acessos ainda não consultados'}
          </KV>
          <Button
            kind="primary"
            disabled={p.busy || !p.connected || !selected || selected.disabled}
            onClick={() => selected && p.enter(selected.id)}
          >
            Abrir empresa
          </Button>
        </aside>
      </div>
    </>
  );
}
function Dashboard({
  ops,
  brand,
  navigate,
  add,
  inventoryAvailable,
  openEquipment,
}: {
  openEquipment?: (id: string) => void;
  inventoryAvailable?: boolean;
  ops: Operations;
  brand: Branding;
  navigate: (p: Page) => void;
  add: () => void;
}) {
  if (ops.inventoryState)
    return (
      <section className="equipment-dashboard">
        <h1>Operação da rede</h1>
        {ops.inventoryState === 'loading' ? (
          <p role="status">Carregando equipamentos…</p>
        ) : ops.inventoryState === 'error' ? (
          <>
            <p role="alert">{ops.inventoryError}</p>
            <Button onClick={ops.retry}>Tentar novamente</Button>
          </>
        ) : (
          <>
            <p className="summary">
              {brand.name} · {ops.devices.length} equipamentos cadastrados ·
              consultas sob demanda
            </p>
            {!ops.devices.length ? (
              <div className="empty-state">
                <h2>Nenhum equipamento cadastrado</h2>
                <Button kind="primary" onClick={add}>
                  Adicionar equipamento
                </Button>
              </div>
            ) : (
              <div
                className="table-scroll"
                tabIndex={0}
                aria-label="Equipamentos da empresa"
              >
                <table className="app-table">
                  <thead>
                    <tr>
                      <th>Equipamento</th>
                      <th>Sistema</th>
                      <th>Estado da leitura</th>
                      <th>Última leitura válida</th>
                      <th>Sessões PPPoE</th>
                    </tr>
                  </thead>
                  <tbody>
                    {ops.devices.map((d) => (
                      <tr key={d.id}>
                        <td>
                          <Button
                            kind="flat device-link"
                            onClick={() => openEquipment?.(d.id)}
                          >
                            {d.name} · {d.ip}
                          </Button>
                        </td>
                        <td>{d.os}</td>
                        <td>{d.state}</td>
                        <td>{d.time}</td>
                        <td>
                          {d.type === 'Concentrador'
                            ? d.pppoeAt
                              ? `${d.sessions ?? 0} ONLINE · ${d.offline ?? 0} OFFLINE · ${new Date(d.pppoeAt).toLocaleString('pt-BR')}`
                              : 'Sem leitura PPPoE'
                            : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <p className="muted">
              Topologia, tráfego, alertas e histórico ainda não estão
              disponíveis. O serviço ativo indica apenas o serviço local.
            </p>
          </>
        )}
      </section>
    );
  if (!ops.devices.length)
    return (
      <Empty
        title={
          inventoryAvailable ? 'Inventário disponível nesta empresa' : undefined
        }
      >
        <Button kind="primary" onClick={add}>
          Adicionar equipamento
        </Button>
      </Empty>
    );
  const concentrators = ops.devices.filter((d) => d.sessions !== undefined);
  const t = ops.traffic;
  return (
    <div className="dashboard-body">
      <div className="dashboard-hero">
        <div>
          <h1>Operação da rede</h1>
          <p className="summary">
            {`${brand.name} · ${ops.devices.length} equipamentos · ${concentrators.length} concentradores`}
          </p>
        </div>
        <Button kind="flat" onClick={() => navigate(6)}>
          Topologia
        </Button>
      </div>
      <div className="operation-strip">
        <div className="operation-stat session-total">
          <span className="number">{ops.total ?? '—'}</span>
          <span className="stat-label">sessões PPPoE ativas</span>
        </div>
        <div className="operation-stat">
          <span className="number">{ops.complete || '—'}</span>
          <span className="stat-label">consultas completas</span>
        </div>
        <div className="operation-stat">
          <span className="number warn">{ops.alerts.length}</span>
          <span className="stat-label">alertas em aberto</span>
        </div>
      </div>
      <div className="dashboard-grid">
        <div className="dashboard-primary">
          <section className="chart-section">
            <div className="section-heading">
              <div>
                <h2>Tráfego da borda</h2>
                <p className="section-context">
                  {t?.source || 'Sem origem disponível'}
                </p>
              </div>
              <div style={{ textAlign: 'right' }}>
                <div className="chart-value">{t?.value || '—'}</div>
                <p className="section-context">Última hora</p>
              </div>
            </div>
            {t ? (
              <div className="traffic-chart">
                <div className="chart-axis" aria-hidden="true">
                  {['750', '500', '250', '0'].map((v) => (
                    <span key={v}>{v}</span>
                  ))}
                </div>
                <svg
                  viewBox="0 0 650 194"
                  preserveAspectRatio="none"
                  role="img"
                  aria-label="Entrada de tráfego ilustrativa, de 13:32 a 14:32, em Mbps"
                >
                  <defs>
                    <linearGradient
                      id="traffic-shade"
                      x1="0"
                      y1="0"
                      x2="0"
                      y2="1"
                    >
                      <stop stopColor="var(--primary)" stopOpacity=".13" />
                      <stop
                        offset="1"
                        stopColor="var(--primary)"
                        stopOpacity="0"
                      />
                    </linearGradient>
                  </defs>
                  <path
                    className="chart-grid"
                    d="M0 12H650M0 69H650M0 126H650M0 182H650"
                  />
                  <path d={t.path + 'V182H0Z'} fill="url(#traffic-shade)" />
                  <path className="chart-line" d={t.path} />
                </svg>
                <div className="chart-times">
                  {t.times.map((v) => (
                    <span key={v}>{v}</span>
                  ))}
                </div>
              </div>
            ) : (
              <p className="muted">Nenhuma leitura de tráfego disponível.</p>
            )}
          </section>
          <section className="concentrators-section">
            <div className="section-heading">
              <h2>Concentradores</h2>
              <Button kind="flat tiny" onClick={() => navigate(4)}>
                Ver equipamentos
              </Button>
            </div>
            <table className="app-table">
              <thead>
                <tr>
                  <th>Equipamento</th>
                  <th>RouterOS</th>
                  <th style={{ textAlign: 'right' }}>Sessões</th>
                  <th style={{ textAlign: 'right' }}>Leitura</th>
                </tr>
              </thead>
              <tbody>
                {concentrators.map((d) => (
                  <tr
                    key={d.id}
                    className="selectable"
                    tabIndex={0}
                    onClick={() => navigate(5)}
                    onKeyDown={(e) => e.key === 'Enter' && navigate(5)}
                  >
                    <td>
                      <span className="cell-name">{d.name}</span>
                      <span className="cell-sub mono">{d.ip}</span>
                    </td>
                    <td>{d.os.replace('RouterOS ', '')}</td>
                    <td style={{ textAlign: 'right' }}>{d.sessions}</td>
                    <td className="reading-time" style={{ textAlign: 'right' }}>
                      {d.time}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        </div>
        <aside className="dashboard-secondary">
          <section className="attention-list">
            <div className="section-heading">
              <h2>Precisam de atenção</h2>
            </div>
            {ops.alerts.map((a) => (
              <button
                key={a.name}
                className="quiet-alert"
                onClick={() => navigate(a.target === 'session' ? 5 : 4)}
              >
                <strong>{a.name}</strong>
                <p className={a.kind}>{a.description}</p>
                <small>
                  {a.timeKind}
                  <br />
                  {a.at}
                </small>
              </button>
            ))}
          </section>
          <section>
            <div className="section-heading">
              <h2>Últimas consultas</h2>
            </div>
            {ops.activity.map((a) => (
              <div key={a.name} className="activity-row">
                <span>{a.name}</span>
                <p>{a.description}</p>
                <small>{a.at}</small>
              </div>
            ))}
          </section>
        </aside>
      </div>
    </div>
  );
}
function Inspector({
  device: d,
  topology = false,
  navigate,
  unavailable,
}: {
  device: Device;
  topology?: boolean;
  navigate: (p: Page) => void;
  unavailable: (s: string) => void;
}) {
  return (
    <>
      <p className="inspector-heading">
        {topology ? 'Equipamento selecionado' : 'Detalhes do equipamento'}
      </p>
      <h2>{d.name}</h2>
      <p className="subtitle">{`${d.type} · ${d.os}`}</p>
      <State kind={d.style}>{d.state}</State>
      <KV label="Endereço" mono>
        {d.ip}
      </KV>
      <KV label="Protocolo">{d.access}</KV>
      <KV label="Última leitura válida">{d.time}</KV>
      <KV
        label={
          d.style === 'good' ? 'Identidade do destino' : 'Resultado da consulta'
        }
      >
        {d.detail}
      </KV>
      {topology && (
        <>
          <div className="details-label">Relações</div>
          {d.relations?.map(([n, o]) => (
            <div className="origin-row" key={n}>
              <span>{n}</span>
              <span className="muted">{o}</span>
            </div>
          ))}
        </>
      )}
      <div className="bottom-actions">
        <Button
          kind="primary"
          onClick={() =>
            d.sessions !== undefined ? navigate(5) : unavailable(d.name)
          }
        >
          Abrir
        </Button>
        <Button kind="flat" onClick={() => unavailable('Editar equipamento')}>
          Editar
        </Button>
      </div>
    </>
  );
}
function Inventory({
  ops,
  navigate,
  unavailable,
  add,
}: {
  ops: Operations;
  navigate: (p: Page) => void;
  unavailable: (s: string) => void;
  add: () => void;
}) {
  const [selected, select] = useState(ops.target || ''),
    [search, setSearch] = useState(''),
    [filter, setFilter] = useState('all');
  if (!ops.devices.length)
    return (
      <Empty>
        <Button kind="primary" onClick={add}>
          Adicionar equipamento
        </Button>
      </Empty>
    );
  const d = ops.devices.find((x) => x.id === selected) || ops.devices[0];
  const list = ops.devices.filter(
    (d) =>
      (d.name + ' ' + d.ip)
        .toLocaleLowerCase()
        .includes(search.toLocaleLowerCase()) &&
      (filter === 'all' || d.style === filter),
  );
  return (
    <div className="pane-layout">
      <section className="pane-main">
        <div className="inventory-tools">
          <input
            placeholder="Buscar equipamento"
            aria-label="Buscar equipamento"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select
            aria-label="Estado dos equipamentos"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          >
            <option value="all">Todos os estados</option>
            <option value="good">Atualizados</option>
            <option value="warn">Revisar leitura</option>
            <option value="bad">Acesso recusado</option>
          </select>
        </div>
        <div className="table-caption">
          {`${ops.devices.length} equipamentos cadastrados`}
        </div>
        <div
          className="table-scroll"
          tabIndex={0}
          aria-label="Lista de equipamentos"
        >
          <table className="app-table inventory-table">
            <thead>
              <tr>
                <th>Equipamento</th>
                <th>Endereço</th>
                <th className="os-cell">Sistema</th>
                <th>Consulta</th>
                <th>Última leitura</th>
              </tr>
            </thead>
            <tbody>
              {list.map((x) => (
                <tr
                  key={x.id}
                  className={'selectable ' + (d.id === x.id ? 'selected' : '')}
                  tabIndex={0}
                  aria-selected={d.id === x.id}
                  onClick={() => select(x.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      select(x.id);
                    }
                  }}
                  onDoubleClick={() => x.sessions !== undefined && navigate(5)}
                >
                  <td>
                    <span className="cell-name">{x.name}</span>
                    <span className="cell-sub">{x.type}</span>
                  </td>
                  <td className="mono">{x.ip}</td>
                  <td className="os-cell">{x.os.replace('RouterOS ', 'v')}</td>
                  <td>
                    <State kind={x.style}>{x.state}</State>
                  </td>
                  <td className="reading-time">{x.time}</td>
                </tr>
              ))}
              {!list.length && (
                <tr>
                  <td
                    colSpan={5}
                    style={{
                      padding: '60px 0',
                      textAlign: 'center',
                      color: 'var(--muted)',
                    }}
                  >
                    Nenhum resultado
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="list-footer">
          <span>{list.length} equipamentos</span>
          <span>Perfil ativo</span>
        </div>
      </section>
      <aside className="inspector">
        <Inspector device={d} navigate={navigate} unavailable={unavailable} />
      </aside>
    </div>
  );
}
function Concentrator({
  ops,
  navigate,
  unavailable,
}: {
  ops: Operations;
  navigate: (p: Page) => void;
  unavailable: (s: string) => void;
}) {
  const [tab, setTab] = useState('pppoe'),
    [selected, select] = useState(0),
    [search, setSearch] = useState(''),
    [sourceOpen, setSourceOpen] = useState(false);
  const d = ops.devices.find((d) => d.id === ops.target),
    s = ops.sessions[selected];
  if (!d)
    return (
      <Empty title="Nenhum equipamento selecionado">
        <Button kind="primary" onClick={() => navigate(4)}>
          Equipamentos
        </Button>
      </Empty>
    );
  return (
    <>
      <div className="pane-layout">
        <section className="pane-main">
          <div className="tabs" role="tablist" aria-label="Concentrador">
            {[
              ['summary', 'Resumo'],
              ['pppoe', 'PPPoE'],
              ['interfaces', 'Interfaces'],
              ['history', 'Histórico'],
            ].map(([id, name]) => (
              <button
                key={id}
                role="tab"
                disabled={id === 'interfaces' || id === 'history'}
                aria-selected={tab === id}
                className={tab === id ? 'selected' : ''}
                onClick={() => setTab(id)}
              >
                {name}
              </button>
            ))}
          </div>
          {tab === 'pppoe' ? (
            <>
              <div className="detail-subhead">
                <div>
                  <h1>Sessões PPPoE</h1>
                  <p className="muted">
                    {`${d.sessions ?? '—'} ativas · ${d.time}`}
                  </p>
                </div>
                <input
                  placeholder="Buscar sessão"
                  aria-label="Buscar sessão"
                  style={{ width: 220 }}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div
                className="table-scroll"
                tabIndex={0}
                aria-label="Lista de sessões"
              >
                <table className="app-table session-table">
                  <thead>
                    <tr>
                      <th>Usuário</th>
                      <th>Endereço</th>
                      <th>Tempo</th>
                      <th>Estado</th>
                    </tr>
                  </thead>
                  <tbody>
                    {ops.sessions.map((s, i) => (
                      <tr
                        key={s.user}
                        hidden={
                          !(s.user + ' ' + s.ip)
                            .toLowerCase()
                            .includes(search.toLowerCase())
                        }
                        className={
                          'selectable ' + (selected === i ? 'selected' : '')
                        }
                        tabIndex={0}
                        aria-selected={selected === i}
                        onClick={() => select(i)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault();
                            select(i);
                          }
                        }}
                      >
                        <td>
                          <span className="cell-name">{s.user}</span>
                        </td>
                        <td className="mono">{s.ip}</td>
                        <td>{s.uptime}</td>
                        <td>
                          <State kind="good">Ativa</State>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="list-footer">
                <span>
                  {`${ops.sessions.length} de ${d.sessions ?? '—'} sessões`}
                </span>
                <span>{d.access}</span>
              </div>
            </>
          ) : (
            <>
              <div className="intro">
                <h1>{d.name}</h1>
                <p>{`${d.type} · ${d.os}`}</p>
              </div>
              <KV label="Tempo ligado">{d.uptime || 'Indisponível'}</KV>
              <KV label="Última consulta">{d.time}</KV>
              <KV label="Identidade">{d.detail}</KV>
              <KV label="Sessões PPPoE">{d.sessions ?? '—'}</KV>
            </>
          )}
        </section>
        <aside className="inspector">
          {tab === 'pppoe' && s ? (
            <>
              <p className="inspector-heading">Sessão selecionada</p>
              <h2>{s.user}</h2>
              <p className="subtitle">{d.name} · PPPoE</p>
              <State kind="good">Sessão ativa</State>
              <KV label="Endereço" mono>
                {s.ip}
              </KV>
              <KV label="Tempo conectado">{s.uptime}</KV>
              <div className="details-label">Rádio</div>
              {s.radio ? (
                <>
                  <div className={'signal-value ' + (s.warning ? 'warn' : '')}>
                    {s.signal ?? '—'}
                    <span>dBm · RX</span>
                  </div>
                  <p className="signal-source">
                    {`${s.radio} · ${s.radioAt || 'Horário indisponível'}`}
                  </p>
                  <div className="small-grid">
                    <KV label="CCQ">
                      <span className="muted">Indisponível</span>
                    </KV>
                    <KV label="Associação">Cadastrada</KV>
                  </div>
                  <KV label="MAC da CPE" mono>
                    {s.mac ?? 'Indisponível'}
                  </KV>
                  <div className="bottom-actions">
                    <Button kind="flat" onClick={() => setSourceOpen(true)}>
                      Origem da leitura
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <div className="kv">
                    <span className="muted">Não associado</span>
                  </div>
                  <div className="bottom-actions">
                    <Button onClick={() => unavailable('Associar rádio')}>
                      Associar rádio
                    </Button>
                  </div>
                </>
              )}
            </>
          ) : (
            <Inspector
              device={d}
              navigate={navigate}
              unavailable={unavailable}
            />
          )}
        </aside>
      </div>
      {sourceOpen && s && (
        <Dialog title="Origem da leitura" close={() => setSourceOpen(false)}>
          <KV label="Sessão PPPoE">{`${d.name} · ${d.access} · ${d.time}`}</KV>
          <KV label="Rádio">
            {s.radio
              ? `${s.radio} · ${s.radioAt || 'Horário indisponível'}`
              : 'Não associado'}
          </KV>
          <KV label="Qualidade">Parcial · CCQ indisponível</KV>
          <KV label="Associação">
            {s.radio ? 'Sessão → CPE → AP · Cadastrada' : 'Não associado'}
          </KV>
        </Dialog>
      )}
    </>
  );
}
function Topology({
  ops,
  navigate,
  unavailable,
  add,
}: {
  ops: Operations;
  navigate: (p: Page) => void;
  unavailable: (s: string) => void;
  add: () => void;
}) {
  const [selected, select] = useState(ops.target || '');
  if (!ops.devices.length)
    return (
      <Empty>
        <Button kind="primary" onClick={add}>
          Adicionar equipamento
        </Button>
      </Empty>
    );
  const d = ops.devices.find((x) => x.id === selected) || ops.devices[0];
  return (
    <div className="map-layout">
      <section
        className="map"
        aria-label={
          'Topologia lógica de ' + ops.devices.length + ' equipamentos'
        }
      >
        <div className="map-canvas">
          <span className="map-overview">
            {`${ops.devices.length} equipamentos · Topologia lógica`}
          </span>
          <svg
            className="wires"
            viewBox="0 0 916 770"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            {ops.links?.map((link, i) => (
              <path key={i} className={link.kind} d={link.path} />
            ))}
          </svg>
          {ops.devices.map((x) => (
            <button
              key={x.id}
              className={'node ' + (x.id === d.id ? 'chosen' : '')}
              style={
                {
                  left: (x.position?.[0] || 0) + 'px',
                  '--node-y': (x.position?.[1] || 0) + 'px',
                } as CSSProperties
              }
              onClick={() => select(x.id)}
              aria-pressed={x.id === d.id}
            >
              <strong>{x.name}</strong>
              <div className="node-address">{x.ip}</div>
              <small className={x.style === 'good' ? '' : x.style}>
                {x.sessions !== undefined
                  ? x.sessions + ' sessões PPPoE'
                  : x.state}
              </small>
            </button>
          ))}
          <span className="map-caption">
            {ops.lastQuery
              ? 'Observações até ' + ops.lastQuery
              : 'Sem observações'}
          </span>
        </div>
      </section>
      <aside className="inspector">
        <Inspector
          device={d}
          topology
          navigate={navigate}
          unavailable={unavailable}
        />
      </aside>
    </div>
  );
}
