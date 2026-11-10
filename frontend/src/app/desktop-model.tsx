import type { AccessErrorKind } from '../features/access/access-error-state';
import type { ReactNode } from 'react';
export type RouteName =
  | 'companies'
  | 'overview'
  | 'equipment'
  | 'adicionar'
  | 'sessao'
  | 'vitrine'
  | 'credentials'
  | 'pppoe'
  | 'topology'
  | 'settings';
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
  adicionar: 4,
  sessao: 4,
  credentials: 4,
  vitrine: 4,
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
  changePassword?: (current: string, next: string) => Promise<boolean>;
  passwordError?: string;
  passwordErrorKind?: AccessErrorKind;
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
