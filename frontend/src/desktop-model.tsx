import type { ReactNode } from 'react';

export type RouteName =
  'companies' | 'overview' | 'equipment' | 'pppoe' | 'topology' | 'settings';

export type LegacyPage = 2 | 3 | 4 | 5 | 6 | 7;
export type Page = RouteName | LegacyPage;

const LEGACY_TO_ROUTE: Record<LegacyPage, RouteName> = {
  2: 'companies',
  3: 'overview',
  4: 'equipment',
  5: 'pppoe',
  6: 'topology',
  7: 'settings',
};

const ROUTE_TO_LEGACY: Record<RouteName, LegacyPage> = {
  companies: 2,
  overview: 3,
  equipment: 4,
  pppoe: 5,
  topology: 6,
  settings: 7,
};

export function normalizeRoute(page: Page): RouteName {
  if (typeof page === 'number') {
    return LEGACY_TO_ROUTE[page] || 'overview';
  }
  return page;
}

export function routeLegacyPageNumber(page: Page): LegacyPage {
  return ROUTE_TO_LEGACY[normalizeRoute(page)];
}

export function routeTitle(route: RouteName): string {
  const titles: Record<RouteName, string> = {
    companies: 'Empresas',
    overview: 'Visão geral',
    equipment: 'Equipamentos',
    pppoe: 'Sessões PPPoE',
    topology: 'Topologia',
    settings: 'Aparência',
  };
  return titles[route];
}

export function routeBackTarget(route: RouteName): RouteName | undefined {
  if (route === 'companies' || route === 'overview') return undefined;
  if (route === 'pppoe') return 'equipment';
  return 'overview';
}

export function routeBreadcrumbs(
  route: RouteName,
  companyName: string,
): Array<{ label: string; route?: RouteName }> {
  const base = companyName || 'Empresa';
  if (route === 'companies') {
    return [{ label: 'Empresas' }];
  }
  if (route === 'overview') {
    return [{ label: base, route: 'companies' }, { label: 'Visão geral' }];
  }
  if (route === 'pppoe') {
    return [
      { label: base, route: 'companies' },
      { label: 'Visão geral', route: 'overview' },
      { label: 'Equipamentos', route: 'equipment' },
      { label: 'Sessões PPPoE' },
    ];
  }
  const labels: Record<RouteName, string> = {
    companies: 'Empresas',
    overview: 'Visão geral',
    equipment: 'Equipamentos',
    pppoe: 'Sessões PPPoE',
    topology: 'Topologia',
    settings: 'Configurações',
  };
  return [
    { label: base, route: 'companies' },
    { label: 'Visão geral', route: 'overview' },
    { label: labels[route] },
  ];
}

export type Branding = { name: string; accent: string; logo: string };
export type Device = {
  id: string;
  name: string;
  ip: string;
  type: string;
  os: string;
  access: string;
  state: string;
  style: string;
  time: string;
  detail: string;
  sessions?: number;
  offline?: number;
  pppoeAt?: string;
  uptime?: string;
  position?: [number, number];
  relations?: [string, string][];
};
export type Session = {
  user: string;
  ip: string;
  uptime: string;
  signal: string | null;
  warning?: boolean;
  radio: string | null;
  radioAt?: string;
  mac: string | null;
};
export type Company = {
  id: string;
  name: string;
  lastAccess?: string;
  connection?: string;
  connectionStyle?: string;
  count?: number;
  network?: string;
  inventory?: string;
  access?: string;
  disabled?: boolean;
};
export type Operations = {
  retry?: () => void;
  inventoryState?: 'loading' | 'ready' | 'error';
  inventoryError?: string;
  devices: Device[];
  sessions: Session[];
  alerts: {
    name: string;
    description: string;
    kind: string;
    at: string;
    timeKind: string;
    target: string;
  }[];
  activity: { name: string; description: string; at: string }[];
  total?: number;
  complete?: string;
  traffic?: { value: string; source: string; path: string; times: string[] };
  network?: string;
  lastQuery?: string;
  target?: string;
  links?: { kind: string; path: string }[];
};
export const emptyOperations: Operations = {
  devices: [],
  sessions: [],
  alerts: [],
  activity: [],
};

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
