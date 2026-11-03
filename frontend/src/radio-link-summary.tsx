import type { RadioLinkState } from './radio-link-state';
import { RadioCodes } from './radio-contract';
import { formatDate } from './network-format';
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
export function RadioStoredLink({ state }: { state: RadioLinkState }) {
  const { radio, link, proposal } = state;
  return (
    <>
      {link ? (
        <>
          <p>
            Vínculo confirmado em {formatDate(link.confirmed_at)} ·{' '}
            {link.origin}
          </p>
          <p>
            {link.mac} · {link.interface} · BSSID {link.bssid}
          </p>
          {!link.confirmed_now && (
            <p role="status">
              Vínculo não confirmado agora. O cadastro foi preservado.
            </p>
          )}
        </>
      ) : (
        !radio.error &&
        !proposal && <p>Nenhum vínculo confirmado para esta sessão.</p>
      )}
    </>
  );
}
