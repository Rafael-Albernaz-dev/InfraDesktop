import { roleGroups } from './equipment-roles';

const labels: Record<string, string> = Object.fromEntries(roleGroups.flatMap(([, roles]) => roles));
export const equipmentType = (role = 'other') => labels[role] || labels.other;
