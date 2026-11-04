import { RadioRecoveryPicker } from './radio-recovery-picker';
import { Button } from './ui/Button';

import {
  createRecoveryChoice,
  createRecoverySave,
  useRecoveryAutoload,
  useRecoveryState,
} from './radio-credential-state';
import type { RadioRecoveryProps } from './radio-credential-state';
export function RadioCredentialRecovery(props: RadioRecoveryProps) {
  const state = useRecoveryState(props);
  const choose = createRecoveryChoice(state),
    save = createRecoverySave(state);
  useRecoveryAutoload(state, choose);
  const { message, open, busy } = state;
  return (
    <div>
      {message && <p role="status">{message}</p>}
      {open ? (
        <RadioRecoveryPicker state={state} save={save} />
      ) : (
        <Button type="submit" disabled={busy} onClick={() => void choose()}>
          Escolher outra
        </Button>
      )}
    </div>
  );
}
