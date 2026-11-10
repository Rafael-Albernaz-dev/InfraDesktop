import {
  RadioLinkDeviceSelect,
  RadioManualIdentityFields,
} from './radio-link-fields';

import { Button } from '../../shared/ui/Button';
import type { RadioLinkState } from './radio-link-state';
import { RadioFamilies, RadioRoles } from './radio-contract';
import { formatMacAddress, formatRadioFamily } from './network-format';
export function RadioLinkProposal({ state }: { state: RadioLinkState }) {
  const { blocked, radio, editing, saving, confirm, link, proposal } = state;
  return (
    <>
      {!link && !editing && !radio.busy && proposal && (
        <div>
          <p>
            AP: {proposal.ap_name} ({proposal.ap_address}) · CPE{' '}
            {proposal.address} ·{' '}
            {proposal.family === RadioFamilies.automatic
              ? 'Família: identificar após a confirmação'
              : formatRadioFamily(proposal.family)}
          </p>
          <p>
            Credencial {proposal.credential_alias} ·{' '}
            {formatMacAddress(proposal.choice.mac)} ·{' '}
            {proposal.choice.interface} · BSSID{' '}
            {formatMacAddress(proposal.choice.bssid)}
          </p>
          <Button
            type="submit"
            className="primary"
            disabled={blocked || saving || !proposal.credential_id}
            onClick={() => void confirm(proposal.choice)}
          >
            Confirmar enlace sugerido
          </Button>
          {!proposal.credential_id && (
            <p>
              Selecione a credencial padrão de CPE em Equipamentos para
              confirmar.
            </p>
          )}
        </div>
      )}
    </>
  );
}
export function RadioLinkEditActions({ state }: { state: RadioLinkState }) {
  const { blocked, radio, reopen, editing, saving, edit, link, proposal } =
    state;
  return (
    <>
      {!radio.busy && !editing && !radio.error && (
        <Button
          type="submit"
          className={link || proposal ? '' : 'primary'}
          disabled={blocked || saving}
          onClick={() => void edit()}
        >
          {link ? 'Revisar vínculo' : 'Vincular CPE e AP'}
        </Button>
      )}
      {!link && radio.error && !radio.busy && (
        <Button type="submit" className="primary" onClick={reopen}>
          Reabrir vínculo armazenado
        </Button>
      )}
    </>
  );
}
export function RadioLegacySuggestions({ state }: { state: RadioLinkState }) {
  const { blocked, radio, editing, saving, confirm, link } = state;
  return (
    <>
      {!link &&
        !editing &&
        !radio.busy &&
        !!radio.view?.suggestions?.length && (
          <div>
            <p>
              Sugestões pela registration table; confirme a identidade antes de
              usar.
            </p>
            {radio.view.suggestions.map((s) => (
              <Button
                type="submit"
                key={s.ap_id + '|' + s.interface + '|' + s.mac}
                disabled={blocked || saving}
                onClick={() =>
                  void confirm({
                    cpe_id: s.cpe_id,
                    ap_id: s.ap_id,
                    mac: s.mac,
                    interface: s.interface,
                    bssid: s.bssid,
                    evidence: s.evidence,
                    origin: 'sugerido-confirmado',
                  })
                }
              >
                Confirmar sugestão {s.mac} · {s.interface}
              </Button>
            ))}
          </div>
        )}
    </>
  );
}
export function RadioManualLinkForm({ state }: { state: RadioLinkState }) {
  const { blocked, choice, editing, setEditing, saving, confirm } = state;
  return (
    <>
      {editing && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void confirm(choice);
          }}
        >
          <fieldset disabled={saving || blocked}>
            <legend>Confirmação do vínculo</legend>
            <RadioLinkDeviceSelect
              state={state}
              field="cpe_id"
              role={RadioRoles.cpe}
              label="CPE"
            />
            <RadioLinkDeviceSelect
              state={state}
              field="ap_id"
              role={RadioRoles.ap}
              label="AP"
            />
            <RadioManualIdentityFields state={state} />
            <p>
              Use MAC e BSSID conferidos no rádio. SSID e nome não provam o
              vínculo.
            </p>
            <Button type="submit" className="primary">
              Confirmar vínculo
            </Button>
            <Button type="button" onClick={() => setEditing(false)}>
              Cancelar
            </Button>
          </fieldset>
        </form>
      )}
    </>
  );
}
