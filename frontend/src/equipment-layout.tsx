import { RadioRoles } from './radio-contract';

const labels: Record<string, string> = {
  concentrator: 'Concentrador',
  [RadioRoles.ap]: 'AP',
  [RadioRoles.cpe]: 'CPE',
  [RadioRoles.ptp]: 'PTP',
  other: 'Outro',
};
export const equipmentType = (role = 'other') => labels[role] || labels.other;
