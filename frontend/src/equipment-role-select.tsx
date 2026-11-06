import type { Device, DeviceInput } from './equipment-api';
import { RadioRoles } from './radio-contract';
import { RoleOptions } from './equipment-roles';
import { Select } from './ui/Select';

const radioRoles: string[] = [RadioRoles.ap, RadioRoles.cpe, RadioRoles.ptp];

export function DeviceRoleSelect({
  draft,
  change,
  device,
  defaults,
  setExisting,
}: {
  draft: DeviceInput;
  change: (values: Partial<DeviceInput>) => void;
  device?: Device;
  defaults: Record<string, string>;
  setExisting: (v: boolean) => void;
}) {
  return (
    <label className="field">
      Papel do equipamento
      <Select
        aria-label="Papel do equipamento"
        value={draft.role}
        onChange={(e) => {
          const role = e.target.value;
          if (radioRoles.includes(role)) {
            if (!device) setExisting(true);
            change({
              role,
              credential_id: defaults[role] || '',
              user: '',
              secret: '',
            });
          } else change({ role });
        }}
      >
        <RoleOptions />
      </Select>
    </label>
  );
}
