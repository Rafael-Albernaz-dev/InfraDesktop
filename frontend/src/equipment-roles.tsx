import { RadioRoles } from './radio-contract';

export const roleGroups: ReadonlyArray<
  readonly [string, ReadonlyArray<readonly [string, string]>]
> = [
  [
    'Rede',
    [
      ['concentrator', 'Concentrador'],
      ['edge', 'Roteador de borda'],
      ['switch', 'Switch'],
    ],
  ],
  [
    'Rádio',
    [
      [RadioRoles.ap, 'AP'],
      ['station', 'ST'],
      [RadioRoles.ptp, 'PTP'],
      [RadioRoles.cpe, 'CPE'],
    ],
  ],
  [
    'Energia',
    [
      ['power-monitor', 'Monitor de energia'],
      ['ups', 'Nobreak'],
      ['rectifier', 'Retificadora ou fonte'],
      ['solar-controller', 'Controlador solar'],
      ['battery-monitor', 'Monitor de bateria'],
    ],
  ],
  ['Outros', [['other', 'Outro']]],
];

export const driverOwners: Record<string, string> = {
  station: 'L07-DRV',
  edge: 'L08-B',
  switch: 'L-REDE',
  'power-monitor': 'L-ENERGIA',
  ups: 'L-ENERGIA',
  rectifier: 'L-ENERGIA',
  'solar-controller': 'L-ENERGIA',
  'battery-monitor': 'L-ENERGIA',
};

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
