import type { Credential, Device, Job } from './presentation';

export type ImportRow = {
  line: number;
  name: string;
  address: string;
  port: string;
  role: string;
  block: string;
  credential: string;
  ignored?: boolean;
};
export type ImportItem = {
  row: ImportRow;
  status: 'ready' | 'pending' | 'duplicate' | 'conflict' | 'ignored';
  reason?: string;
  credential_id?: string;
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
    credential_defaults?: Record<string, string>;
    blocks?: string[];
    models?: string[];
    bulk_receipt?: { items: { id: string; revision: number }[] };
    import_items?: ImportItem[];
    import_receipt?: { created: number; ignored: number };
  };
};
