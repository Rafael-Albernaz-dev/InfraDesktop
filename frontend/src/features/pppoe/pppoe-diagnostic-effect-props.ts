import type { DiagState, Scope } from './pppoe-diag-state';
import type { PPResult } from '../../platform/wails/presentation';
export type UseDiagLoadEffectProps = {
  s: DiagState;
  context: string;
  readKey: string;
  profile: string;
  activation: string;
  deviceId: string;
  scope: Scope;
  sourceObservation?: string;
  sourceRow?: string;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  readSequenceRef: React.MutableRefObject<number>;
  active: (generation: number, result?: PPResult) => boolean;
  acknowledge: (op?: string) => void;
};
export type UseDiagPageEffectProps = {
  s: DiagState;
  context: string;
  diagnosticID?: string;
  kind: string;
  profile: string;
  activation: string;
  deviceId: string;
  scope: Scope;
  readKey: string;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  active: (generation: number, result?: PPResult) => boolean;
};
export type UseDiagPollEffectProps = {
  s: DiagState;
  context: string;
  profile: string;
  activation: string;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  active: (generation: number, result?: PPResult) => boolean;
  acknowledge: (op?: string) => void;
};
