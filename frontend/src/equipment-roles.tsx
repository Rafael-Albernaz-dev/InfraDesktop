import { roleGroups } from './equipment-role-options';

export function RoleOptions() {
  return roleGroups.map(([group, roles]) => (
    <optgroup key={group} label={group}>
      {roles.map(([value, label]) => (
        <option key={value} value={value}>
          {label}
        </option>
      ))}
    </optgroup>
  ));
}
