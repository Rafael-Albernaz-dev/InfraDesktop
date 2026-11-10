import {
  CancelPPPoECollection,
  CancelPPPoEDiagnostic,
  CancelPPPoERead,
  CancelPPPoEReadOperation,
  ComparePPPoE,
  DeletePPPoEReference,
  ExportPPPoE,
  ExportPPPoEOperational,
  GetDeviceMonitoring,
  GetPPPoECurrent,
  GetPPPoEDiagnostic,
  GetPPPoEObservation,
  GetPPPoESession,
  ListPPPoEDiagnostic,
  ListPPPoEReferences,
  ListPPPoESessions,
  PPPoECollectionStatus,
  PPPoEContextCounts,
  PPPoEDiagnosticStatus,
  PPPoEReadPage,
  PPPoEReadStatus,
  SavePPPoEReference,
  StartPPPoECatalog,
  StartPPPoECollection,
  StartPPPoEPing,
  StartPPPoERadius,
  StartPPPoERead,
  StopPPPoETraffic,
} from '../../../wailsjs/go/presentation/App';
import type { equipment } from '../../../wailsjs/go/models';
import type { PPQuery, PPResult } from './pppoe-types';
// Aridade fixada pelo método Go do binding; a tupla rotulada documenta cada posição.
type DiagnosticListArgs = [
  profile: string,
  activation: string,
  device: string,
  scope: string,
  observation: string,
  cursor: string,
  kind?: string,
];
type SessionListArgs = [
  profile: string,
  activation: string,
  device: string,
  observation: string,
  query: PPQuery,
  cursor: string,
];
type CompareArgs = [
  profile: string,
  activation: string,
  device: string,
  base: string,
  target: string,
  query: PPQuery,
  cursor: string,
];
const pdBindings = {
  catalog: StartPPPoECatalog,
  radius: StartPPPoERadius,
  ping: StartPPPoEPing,
  status: PPPoEDiagnosticStatus,
  cancel: CancelPPPoEDiagnostic,
  current: GetPPPoEDiagnostic,
  list: (...[p, a, d, s, o, c, kind = '']: DiagnosticListArgs) =>
    ListPPPoEDiagnostic(p, a, d, s, o, c, kind),
};
export const pppoeDiagnosticAPI: {
  [K in keyof typeof pdBindings]: (
    ...args: Parameters<(typeof pdBindings)[K]>
  ) => Promise<PPResult>;
} = pdBindings;
const ppBindings = {
  start: StartPPPoECollection,
  status: PPPoECollectionStatus,
  cancel: CancelPPPoECollection,
  current: GetPPPoECurrent,
  observation: GetPPPoEObservation,
  list: (...[p, a, d, o, q, c]: SessionListArgs) =>
    ListPPPoESessions(
      p,
      a,
      d,
      o,
      { ...q, category: '' } as equipment.PPQuery,
      c,
    ),
  detail: GetPPPoESession,
  groups: PPPoEContextCounts,
  save: SavePPPoEReference,
  references: ListPPPoEReferences,
  remove: DeletePPPoEReference,
  compare: (...[p, a, d, b, t, q, c]: CompareArgs) =>
    ComparePPPoE(
      p,
      a,
      d,
      b,
      t,
      {
        ...q,
        search: '',
        sort: '',
        desc: false,
        duplicates_only: false,
      } as equipment.PPQuery,
      c,
    ),
};
export const pppoeAPI: {
  [K in keyof typeof ppBindings]: (
    ...args: Parameters<(typeof ppBindings)[K]>
  ) => Promise<PPResult>;
} & {
  export: typeof ExportPPPoE;
  exportOperational: typeof ExportPPPoEOperational;
} = {
  ...ppBindings,
  export: ExportPPPoE,
  exportOperational: ExportPPPoEOperational,
};
const readBindings = {
  start: StartPPPoERead,
  stop: StopPPPoETraffic,
  status: PPPoEReadStatus,
  cancel: CancelPPPoERead,
  cancelOperation: CancelPPPoEReadOperation,
  page: PPPoEReadPage,
};
export const pppoeReadAPI: {
  [K in keyof typeof readBindings]: (
    ...args: Parameters<(typeof readBindings)[K]>
  ) => Promise<PPResult>;
} = readBindings;
export const deviceMonitoringAPI = {
  current: GetDeviceMonitoring as (
    ...args: Parameters<typeof GetDeviceMonitoring>
  ) => Promise<PPResult>,
};
