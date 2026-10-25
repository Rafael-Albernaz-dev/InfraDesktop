import {
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
