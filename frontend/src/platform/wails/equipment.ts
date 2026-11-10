import {
  BulkUpdateEquipment,
  CancelQuery,
  ConfirmTrust,
  DeleteCredential,
  EnrollmentStatus,
  GetEquipment,
  InspectTrust,
  ListCredentials,
  ListEquipment,
  ListEquipmentFiltered,
  PutCredential,
  QueryStatus,
  ResetTrust,
  SaveEquipment,
  SetCredentialDefault,
  SetEquipmentActive,
  StartQuery,
} from '../../../wailsjs/go/presentation/App';
import type { equipment, presentation } from '../../../wailsjs/go/models';
import { createInventoryBindings } from '../../features/inventory/inventory-bindings';
import type { EquipmentResult } from '../../features/inventory/inventory-import-types';
import type { DeviceHealth } from './pppoe-types';
export type Observation = Omit<equipment.Observation, 'convertValues'>;
type DeviceProjection = {
  live?: {
    cpu?: number;
    clients?: number;
    uptime_seconds?: number;
    at?: string;
    fault: string;
  };
  monitoring?: DeviceHealth;
  monitoring_at?: string;
  role?: string;
  result?: Observation;
};
export type Device = Omit<
  equipment.Device,
  'convertValues' | keyof DeviceProjection
> &
  DeviceProjection;

export type Credential = Omit<equipment.Credential, 'convertValues'>;
export type Job = Omit<equipment.Job, 'convertValues' | 'result'> & {
  result?: Observation;
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
  ...createInventoryBindings(
    ListEquipment,
    (p, a, input) =>
      ListEquipmentFiltered(p, a, input as presentation.InventoryFilters),
    (p, a, o, input) =>
      BulkUpdateEquipment(p, a, o, input as equipment.BulkUpdate),
  ),
  enrollmentStatus: EnrollmentStatus,
  get: GetEquipment,
  save: (profile: string, activation: string, input: DeviceInput) =>
    SaveEquipment(profile, activation, input as presentation.DeviceInput),
  active: SetEquipmentActive,
  credentials: ListCredentials,
  deleteCredential: DeleteCredential,
  setDefault: SetCredentialDefault,
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
