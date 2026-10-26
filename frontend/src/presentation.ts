import {
  StartPPPoECollection,
  PPPoECollectionStatus,
  CancelPPPoECollection,
  GetPPPoECurrent,
  GetPPPoEObservation,
  ListPPPoESessions,
  GetPPPoESession,
  SavePPPoEReference,
  ListPPPoEReferences,
  DeletePPPoEReference,
  ComparePPPoE,
  ExportPPPoE,
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
export type Device = Omit<equipment.Device, 'convertValues' | 'result'> & {
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
export type DeviceInput = Omit<presentation.DeviceInput, 'convertValues'>;
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
export type PPRow = {
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
  base_row?: string;
  target_row?: string;
  category: string;
  reason: string;
  address_changed: boolean;
  caller_changed: boolean;
};
export type PPResult = {
  code: string;
  profile_id: string;
  activation_id: string;
  error?: { reason?: string; entity_id?: string };
  data?: {
    effect_state?: string;
    current?: PPObservation;
    rows?: PPRow[];
    references?: PPReference[];
    deltas?: PPDelta[];
    total: number;
    next_cursor?: string;
    job?: { id: string; state: string; code: string };
    attempt?: {
      state: string;
      code: string;
      quality: { status: string; counts_known: boolean; eligible: number };
    };
  };
};
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
} & { export: typeof ExportPPPoE } = { ...ppBindings, export: ExportPPPoE };
