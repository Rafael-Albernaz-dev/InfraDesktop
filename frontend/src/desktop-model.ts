export type Page = 2 | 3 | 4 | 5 | 6 | 7;
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
