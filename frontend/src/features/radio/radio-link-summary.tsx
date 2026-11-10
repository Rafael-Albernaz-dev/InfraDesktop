import { Button } from '../../shared/ui/Button';
import type { RadioLinkState } from './radio-link-state';

import {
  formatDate,
  formatLinkOrigin,
  formatMacAddress,
} from '../equipment/network-format';

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
                <Button
                  type="button"
                  onClick={() => onOpenEquipment(link.cpe_id)}
                >
                  Abrir CPE vinculada
                </Button>
              )}
              {link.ap_id && (
                <Button
                  type="button"
                  onClick={() => onOpenEquipment(link.ap_id)}
                >
                  Abrir AP vinculado
                </Button>
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

export { RadioLinkCredentialErrors } from './radio-link-credential-errors';
