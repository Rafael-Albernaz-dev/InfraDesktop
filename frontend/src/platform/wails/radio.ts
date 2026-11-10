import {
  CancelRadio,
  CancelRadioOperation,
  ConfirmRadioFamily,
  ConfirmRadioLink,
  GetRadio,
  GetRadioIndexPage,
  GetRadioLink,
  RadioStatus,
  StartRadio,
} from '../../../wailsjs/go/presentation/App';
import type { equipment } from '../../../wailsjs/go/models';
import type {
  RadioResult,
  RadioInput,
  RadioLinkChoice,
} from '../../features/equipment/radio-api';
// Aridade fixada pelo método Go do binding; a tupla rotulada documenta cada posição.
type LinkChoiceArgs = [
  profile: string,
  activation: string,
  device: string,
  observation: string,
  row: string,
  operation: string,
  choice: RadioLinkChoice,
];
export const radioAPI = {
  indexPage: GetRadioIndexPage as (
    ...args: Parameters<typeof GetRadioIndexPage>
  ) => Promise<RadioResult>,
  current: (
    p: string,
    a: string,
    d: string,
    r = '',
    offset = 0,
  ): Promise<RadioResult> => GetRadio(p, a, d, r, offset),
  start: (p: string, a: string, input: RadioInput): Promise<RadioResult> =>
    StartRadio(p, a, input as equipment.RadioInput),
  status: (
    p: string,
    a: string,
    j: string,
    r = '',
    offset = 0,
  ): Promise<RadioResult> => RadioStatus(p, a, j, r, offset),
  cancel: (p: string, a: string, j: string): Promise<RadioResult> =>
    CancelRadio(p, a, j),
  cancelOperation: (
    p: string,
    a: string,
    d: string,
    o: string,
  ): Promise<RadioResult> => CancelRadioOperation(p, a, d, o),
  confirmFamily: ConfirmRadioFamily as (
    ...args: Parameters<typeof ConfirmRadioFamily>
  ) => Promise<RadioResult>,
  link: (
    p: string,
    a: string,
    d: string,
    o: string,
    row: string,
  ): Promise<RadioResult> => GetRadioLink(p, a, d, o, row),
  confirmLink: (
    ...[p, a, d, o, row, operation, choice]: LinkChoiceArgs
  ): Promise<RadioResult> =>
    ConfirmRadioLink(
      p,
      a,
      d,
      o,
      row,
      operation,
      choice as equipment.RadioLinkChoice,
    ),
};
