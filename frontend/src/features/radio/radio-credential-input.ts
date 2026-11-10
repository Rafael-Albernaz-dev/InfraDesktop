import type { Device } from '../equipment/equipment-api';
import { RadioTransports } from './radio-contract';
export function credentialChangeInput(target: Device, credential: string) {
  return {
    id: target.id,
    revision: target.revision,
    operation_id: crypto.randomUUID(),
    role: target.role,
    name: target.name,
    block: target.block || '',
    address: target.address,
    port: target.port,
    transport: RadioTransports.ssh,
    credential_id: credential,
    user: '',
    secret: '',
  };
}
