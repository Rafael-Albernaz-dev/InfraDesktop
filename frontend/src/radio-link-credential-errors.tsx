import type { RadioLinkState } from './radio-link-state';
import { RadioCodes } from './radio-contract';

import { RadioCredentialRecovery } from './radio-credential-recovery';

export function RadioLinkCredentialErrors({
  state,
}: {
  state: RadioLinkState;
}) {
  const { profile, activation, radio, changed } = state;
  return (
    <>
      {radio.error && <p role="alert">{radio.error}</p>}
      {radio.view?.ap_code === RadioCodes.REMOTE_AUTH_FAILED &&
        radio.view.link && (
          <RadioCredentialRecovery
            profile={profile}
            activation={activation}
            deviceID={radio.view.link.ap_id}
            changed={changed}
          />
        )}
      {radio.view?.cpe_code === RadioCodes.REMOTE_AUTH_FAILED &&
        radio.view.link && (
          <RadioCredentialRecovery
            profile={profile}
            activation={activation}
            deviceID={radio.view.link.cpe_id}
            changed={changed}
          />
        )}
    </>
  );
}

export function RadioIndexCredentialErrors({
  state,
}: {
  state: RadioLinkState;
}) {
  const { profile, activation, radio, changed, index } = state;
  return (
    <>
      {!radio.busy &&
        index &&
        Object.entries(index.targets)
          .filter(([, code]) => code === RadioCodes.REMOTE_AUTH_FAILED)
          .map(([target]) => (
            <div key={target}>
              <p>AP {index.labels?.[target] || 'cadastrado'}</p>
              <RadioCredentialRecovery
                profile={profile}
                activation={activation}
                deviceID={target}
                changed={changed}
              />
            </div>
          ))}
    </>
  );
}
