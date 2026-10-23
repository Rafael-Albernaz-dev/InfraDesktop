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
} from '../wailsjs/go/presentation/App';
import type { Branding } from './desktop-model';
import type { presentation } from '../wailsjs/go/models';
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
