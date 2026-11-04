import type { RadioEquipmentState } from './radio-equipment-state';
import { RadioCodes } from './radio-contract';
import { radioMessage } from './radio-messages';
export function equipmentRadioError(state: RadioEquipmentState) {
  return state.credentialRefused
    ? radioMessage(RadioCodes.REMOTE_AUTH_FAILED)
    : state.radio.error;
}
