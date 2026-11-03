import type { RadioLinkState } from './radio-link-state';
import { RadioCodes } from './radio-contract';
import {
  formatDate,
  formatLinkOrigin,
  formatMacAddress,
} from './network-format';
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
  const { radio, link, proposal, onOpenEquipment } = state;
  return (
    <>
      {link ? (
        <>
          <p>
            Vínculo confirmado em {formatDate(link.confirmed_at)} ·{' '}
            {formatLinkOrigin(link.origin)}
          </p>
          <p>
            {formatMacAddress(link.mac)} · {link.interface} · BSSID{' '}
            {formatMacAddress(link.bssid)}
          </p>
          {onOpenEquipment && (link.cpe_id || link.ap_id) && (
            <div className="radio-cross-links">
              {link.cpe_id && (
                <button
                  type="button"
                  onClick={() => onOpenEquipment(link.cpe_id)}
                >
                  Abrir CPE vinculada
                </button>
              )}
              {link.ap_id && (
                <button
                  type="button"
                  onClick={() => onOpenEquipment(link.ap_id)}
                >
                  Abrir AP vinculado
                </button>
              )}
            </div>
          )}
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
