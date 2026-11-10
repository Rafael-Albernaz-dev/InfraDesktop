import {
  Status,
  Register,
  Login,
  Recover,
  ResetAccount,
  ChangePassword,
  Logout,
  CreateCompany,
  SelectCompany,
  SetTheme,
  SetInventoryColumns,
  RenewRecovery,
  CopyRecovery,
  AckRecovery,
  SaveRecovery,
  GetBrand,
  SaveBrand,
} from '../../../wailsjs/go/presentation/App';
import type { presentation } from '../../../wailsjs/go/models';
import type { Branding } from '../../features/access/desktop-model';
export type Result = Omit<
  presentation.Result,
  'convertValues' | 'product_version'
> & { product_version?: string; error_reason?: string };
export const access: {
  status: () => Promise<Result>;
  register: (name: string, password: string) => Promise<Result>;
  login: (name: string, password: string, remember: boolean) => Promise<Result>;
  recover: (name: string, code: string, password: string) => Promise<Result>;
  resetAccount: (name: string, password: string) => Promise<Result>;
  changePassword: (current: string, next: string) => Promise<Result>;
  logout: () => Promise<Result>;
  createCompany: (name: string) => Promise<Result>;
  selectCompany: (id: string) => Promise<Result>;
  setTheme: (theme: string) => Promise<Result>;
  setColumns: (columns: string[]) => Promise<Result>;
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
  resetAccount: ResetAccount,
  changePassword: ChangePassword,
  logout: Logout,
  createCompany: CreateCompany,
  selectCompany: SelectCompany,
  setTheme: SetTheme,
  setColumns: SetInventoryColumns,
  renewRecovery: RenewRecovery,
  copyRecovery: CopyRecovery,
  ackRecovery: AckRecovery,
  saveRecovery: SaveRecovery,
  getBrand: GetBrand,
  saveBrand: (profile, activation, brand) =>
    SaveBrand(profile, activation, brand.name, brand.logo, brand.accent),
};
