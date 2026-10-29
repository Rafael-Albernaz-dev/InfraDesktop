import {
  StartPPPoECollection,
  StartPPPoERead,
  GetDeviceMonitoring,
  StopPPPoETraffic,
  PPPoEReadStatus,
  CancelPPPoERead,
  CancelPPPoEReadOperation,
  PPPoEReadPage,
  StartPPPoECatalog,
  StartPPPoERadius,
  StartPPPoEPing,
  PPPoEDiagnosticStatus,
  CancelPPPoEDiagnostic,
  GetPPPoEDiagnostic,
  ListPPPoEDiagnostic,
  PPPoECollectionStatus,
  CancelPPPoECollection,
  GetPPPoECurrent,
  GetPPPoEObservation,
  ListPPPoESessions,
  PPPoEContextCounts,
  GetPPPoESession,
  SavePPPoEReference,
  ListPPPoEReferences,
  DeletePPPoEReference,
  ComparePPPoE,
  ExportPPPoE,
  ExportPPPoEOperational,
  Status,
  Register,
  Login,
  Recover,
  Logout,
  CreateCompany,
  SelectCompany,
  SetTheme,
  RenewRecovery,
  CopyRecovery,
  AckRecovery,
  SaveRecovery,
  GetBrand,
  SaveBrand,
  CancelQuery,
  ConfirmTrust,
  ResetTrust,
  GetEquipment,
  EnrollmentStatus,
  InspectTrust,
  ListCredentials,
  ListEquipment,
  PutCredential,
  QueryStatus,
  SaveEquipment,
  SetEquipmentActive,
  StartQuery,
} from '../wailsjs/go/presentation/App';
import type { Branding } from './desktop-model';
import type { equipment, presentation } from '../wailsjs/go/models';
export type Result = Omit<
  presentation.Result,
  'convertValues' | 'product_version'
> & { product_version?: string };
export const access: {
  status: () => Promise<Result>;
  register: (name: string, password: string) => Promise<Result>;
  login: (name: string, password: string, remember: boolean) => Promise<Result>;
  recover: (name: string, code: string, password: string) => Promise<Result>;
  logout: () => Promise<Result>;
  createCompany: (name: string) => Promise<Result>;
  selectCompany: (id: string) => Promise<Result>;
  setTheme: (theme: string) => Promise<Result>;
  renewRecovery: (password: string, replacePending: boolean) => Promise<Result>;
  copyRecovery: () => Promise<string>;
  ackRecovery: () => Promise<Result>;
  saveRecovery: () => Promise<string>;
  getBrand: (profile: string, activation: string) => Promise<Result>;
  saveBrand: (
    profile: string,
    activation: string,
    brand: Branding,
  ) => Promise<Result>;
} = {
  status: Status,
  register: Register,
  login: Login,
  recover: Recover,
  logout: Logout,
  createCompany: CreateCompany,
  selectCompany: SelectCompany,
  setTheme: SetTheme,
  renewRecovery: RenewRecovery,
  copyRecovery: CopyRecovery,
  ackRecovery: AckRecovery,
  saveRecovery: SaveRecovery,
  getBrand: GetBrand,
  saveBrand: (profile, activation, brand) =>
    SaveBrand(profile, activation, brand.name, brand.logo, brand.accent),
};

export type Observation = Omit<equipment.Observation, 'convertValues'>;
export type Device = Omit<
  equipment.Device,
  'convertValues' | 'result' | 'role' | 'monitoring'
> & {
  monitoring?: DeviceHealth;
  monitoring_at?: string;
  role?: string;
  result?: Observation;
};
export type Credential = Omit<equipment.Credential, 'convertValues'>;
export type Job = Omit<equipment.Job, 'convertValues' | 'result'> & {
  result?: Observation;
};
export type EquipmentResult = {
  code: string;
  profile_id: string;
  activation_id: string;
  data?: {
    devices: Device[];
    credentials: Credential[];
    device?: Device;
    credential?: Credential;
    job?: Job;
    next_cursor: number;
    vault_available: boolean;
    total_devices?: number;
    operation_state?: string;
  };
};
export type DeviceInput = Omit<
  presentation.DeviceInput,
  'convertValues' | 'role'
> & { role?: string };
export type CredentialInput = Omit<
  presentation.CredentialInput,
  'convertValues'
>;
const equipmentBindings = {
  list: ListEquipment,
  enrollmentStatus: EnrollmentStatus,
  get: GetEquipment,
  save: (profile: string, activation: string, input: DeviceInput) =>
    SaveEquipment(profile, activation, input as presentation.DeviceInput),
  active: SetEquipmentActive,
  credentials: ListCredentials,
  putCredential: (
    profile: string,
    activation: string,
    input: CredentialInput,
  ) =>
    PutCredential(profile, activation, input as presentation.CredentialInput),
  inspect: InspectTrust,
  confirm: ConfirmTrust,
  resetTrust: ResetTrust,
  start: StartQuery,
  status: QueryStatus,
  cancel: CancelQuery,
};

export const equipmentAPI: {
  [K in keyof typeof equipmentBindings]: (
    ...args: Parameters<(typeof equipmentBindings)[K]>
  ) => Promise<EquipmentResult>;
} = equipmentBindings;

export type PPQuery = {
  search: string;
  duplicates_only: boolean;
  sort: string;
  desc: boolean;
  category: string;
};
export type PPInterval = { min: number; max: number; known: boolean };
export type PPContext = {
  association: string;
  service?: string;
  interface?: string;
  vlan?: number;
  ipv6?: boolean;
  delegated_prefix?: string;
  pd_status?: string;
  profile?: string;
  secret_state?: string;
  upload_limit?: string;
  download_limit?: string;
};
export type PPGroup = {
  service: string;
  vlan?: number;
  count: number;
  peak_7d?: number;
  history_limited?: boolean;
};
export type PPRow = {
  context?: PPContext;
  state?: string;
  absent_start?: string;
  absent_end?: string;
  row_id: string;
  remote_id: string;
  session_id: string;
  login: string;
  address: string;
  caller_id: string;
  uptime: PPInterval;
  duplicates: number;
};
export type PPObservation = {
  context_omitted?: boolean;
  online?: number;
  offline?: number;
  observation_id: string;
  count: number;
  provenance: { ended_at: string };
  quality: { status: string };
};
export type PPReference = {
  reference_id: string;
  observation_id: string;
  created_at: string;
  count: number;
};
export type PPDelta = {
  base?: Pick<PPRow, 'login' | 'address' | 'caller_id' | 'uptime' | 'state'>;
  target?: Pick<PPRow, 'login' | 'address' | 'caller_id' | 'uptime' | 'state'>;
  base_row?: string;
  target_row?: string;
  category: string;
  reason: string;
  address_changed: boolean;
  caller_changed: boolean;
};
export type PPRate = { min: number; max: number };
export type PPTraffic = {
  points: { t: number; d?: number; u?: number; gap?: boolean }[];
  download_bytes: string;
  upload_bytes: string;
  covered_ns: number;
  covered_min_ns?: number;
  gaps: number;
  overflow?: boolean;
  average_download?: PPRate;
  average_upload?: PPRate;
  peak_download?: PPRate;
  peak_upload?: PPRate;
};
export type DeviceHealth = {
  fasttrack_rules?: number;
  started_at?: string;
  at?: string;
  cpu?: number;
  free_memory?: number;
  total_memory?: number;
  uptime: PPInterval;
  version?: string;
  temperature?: number;
  clock_offset: PPInterval;
  ntp_enabled?: boolean;
  ntp_status?: string;
  error_count?: number;
};
export type DeviceMonitoringPoint = {
  at: string;
  resolution: number;
  session_at?: string;
  online_min?: number;
  online_max?: number;
  online?: number;
  health?: DeviceHealth;
  health_at?: string;
  gap: boolean;
  drop: boolean;
  samples: number;
};
export type DevicePort = {
  running?: boolean;
  speed_bps?: number;
  full_duplex?: boolean;
  link_downs?: string;
};
export type PPPeerSession = {
  device_id: string;
  name: string;
  endpoint: string;
  address: string;
  caller_id: string;
  uptime: PPInterval;
  at: string;
};
export type PPReadRow = {
  duplicate?: { a: PPPeerSession; b: PPPeerSession };
  port?: DevicePort;
  before_port?: DevicePort;
  traffic?: PPTraffic;
  source_at?: string;
  health?: DeviceHealth;
  point?: DeviceMonitoringPoint;
  value?: string;
  link_downs?: string;
  secret?: {
    disabled?: boolean;
    service?: string;
    profile?: string;
    last_logged_out?: string;
    last_caller_id?: string;
    last_disconnect_reason?: string;
    comment?: string;
  };
  kind: string;
  row_id?: string;
  login?: string;
  name?: string;
  download?: PPRate;
  upload?: PPRate;
  uptime: PPInterval;
  at?: string;
  event?: string;
  used?: number;
  size?: number;
  tx?: PPRate;
  rx?: PPRate;
  running?: boolean;
  disabled?: boolean;
  rx_errors?: string;
  tx_errors?: string;
  rx_drops?: string;
  tx_drops?: string;
};
export type PPReadResult = {
  version_divergence?: boolean;
  package_divergence?: boolean;
  compared_devices?: number;
  availability?: Record<string, string>;
  pools_available?: boolean;
  uplinks_available?: boolean;
  pool_total?: number;
  uplink_total?: number;
  scope: string;
  started_at: string;
  ended_at: string;
  duration_ns: number;
  rows: PPReadRow[];
  remote_version?: string;
  cost?: {
    ssh_commands: number;
    ssh_bytes: number;
    ssh_handshake_ns?: number;
    collector_ns?: number;
    first_tunnels: number;
    last_tunnels: number;
    cpu_start?: number;
    cpu_end?: number;
  };
};
export type PPResult = {
  code: string;
  profile_id: string;
  activation_id: string;
  error?: { reason?: string; entity_id?: string };
  data?: {
    effect_state?: string;
    current?: PPObservation;
    diagnostic?: PDDiagnostic;
    diagnostic_rows?: PDRow[];
    rows?: PPRow[];
    groups?: PPGroup[];
    read?: PPReadResult;
    references?: PPReference[];
    deltas?: PPDelta[];
    total: number;
    next_cursor?: string;
    job?: {
      id: string;
      state: string;
      code: string;
      pppoe?: { operation_id?: string };
    };
    attempt?: {
      job_id?: string;
      operation_id?: string;
      source_observation?: string;
      source_row?: string;
      state: string;
      code: string;
      quality: { status: string; counts_known: boolean; eligible: number };
    };
  };
};
export type PDDiagnostic = {
  absent_interfaces?: number;
  absences_limited?: boolean;
  diagnostic_id: string;
  device_id: string;
  scope: string;
  provenance: { ended_at: string; version: string };
  quality: { status: string };
  source_observation?: string;
  source_row?: string;
  count: number;
};
export type PDRow = {
  not_seen_since?: string;
  id: string;
  kind: string;
  name?: string;
  parent?: string;
  parent_id?: string;
  relation?: string;
  type?: string;
  running?: boolean;
  disabled?: boolean;
  dynamic?: boolean;
  invalid?: boolean;
  vlan_id?: number;
  range?: string;
  service_tag?: boolean;
  accept_untagged?: boolean;
  address?: string;
  services?: string;
  auth_port?: number;
  accounting_port?: number;
  protocol?: string;
  use_radius?: boolean;
  accounting?: boolean;
  counters?: {
    pending: string;
    requests: string;
    accepts: string;
    rejects: string;
    resends: string;
    timeouts: string;
    bad_replies: string;
    last_rtt: PPInterval;
  };
  ping?: {
    jitter_ns?: number;
    baseline_median_ns?: number;
    baseline_samples?: number;
    sent: number;
    received: number;
    loss_percent: number;
    min_ns?: number;
    avg_ns?: number;
    max_ns?: number;
    context: string;
    probes: { sequence: number; status: string; rtt_ns?: number }[];
  };
};
const pdBindings = {
  catalog: StartPPPoECatalog,
  radius: StartPPPoERadius,
  ping: StartPPPoEPing,
  status: PPPoEDiagnosticStatus,
  cancel: CancelPPPoEDiagnostic,
  current: GetPPPoEDiagnostic,
  list: (
    p: string,
    a: string,
    d: string,
    s: string,
    o: string,
    c: string,
    kind = '',
  ) => ListPPPoEDiagnostic(p, a, d, s, o, c, kind),
};
export const pppoeDiagnosticAPI: {
  [K in keyof typeof pdBindings]: (
    ...args: Parameters<(typeof pdBindings)[K]>
  ) => Promise<PPResult>;
} = pdBindings;
const ppBindings = {
  start: StartPPPoECollection,
  status: PPPoECollectionStatus,
  cancel: CancelPPPoECollection,
  current: GetPPPoECurrent,
  observation: GetPPPoEObservation,
  list: (p: string, a: string, d: string, o: string, q: PPQuery, c: string) =>
    ListPPPoESessions(
      p,
      a,
      d,
      o,
      { ...q, category: '' } as equipment.PPQuery,
      c,
    ),
  detail: GetPPPoESession,
  groups: PPPoEContextCounts,
  save: SavePPPoEReference,
  references: ListPPPoEReferences,
  remove: DeletePPPoEReference,
  compare: (
    p: string,
    a: string,
    d: string,
    b: string,
    t: string,
    q: PPQuery,
    c: string,
  ) =>
    ComparePPPoE(
      p,
      a,
      d,
      b,
      t,
      {
        ...q,
        search: '',
        sort: '',
        desc: false,
        duplicates_only: false,
      } as equipment.PPQuery,
      c,
    ),
};
export const pppoeAPI: {
  [K in keyof typeof ppBindings]: (
    ...args: Parameters<(typeof ppBindings)[K]>
  ) => Promise<PPResult>;
} & {
  export: typeof ExportPPPoE;
  exportOperational: typeof ExportPPPoEOperational;
} = {
  ...ppBindings,
  export: ExportPPPoE,
  exportOperational: ExportPPPoEOperational,
};

const readBindings = {
  start: StartPPPoERead,
  stop: StopPPPoETraffic,
  status: PPPoEReadStatus,
  cancel: CancelPPPoERead,
  cancelOperation: CancelPPPoEReadOperation,
  page: PPPoEReadPage,
};
export const pppoeReadAPI: {
  [K in keyof typeof readBindings]: (
    ...args: Parameters<(typeof readBindings)[K]>
  ) => Promise<PPResult>;
} = readBindings;

export const deviceMonitoringAPI = {
  current: GetDeviceMonitoring as (
    ...args: Parameters<typeof GetDeviceMonitoring>
  ) => Promise<PPResult>,
};
