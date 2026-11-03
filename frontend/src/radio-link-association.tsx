import type { RadioLinkState } from './radio-link-state';
import { RadioFamilies } from './radio-contract';
import { RadioRoles } from './radio-contract';
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
              : proposal.family}
          </p>
          <p>
            Credencial {proposal.credential_alias} · {proposal.choice.mac} ·{' '}
            {proposal.choice.interface} · BSSID {proposal.choice.bssid}
          </p>
          <button
            className="primary"
            disabled={blocked || saving || !proposal.credential_id}
            onClick={() => void confirm(proposal.choice)}
          >
            Confirmar enlace sugerido
          </button>
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
        <button
          className={link || proposal ? '' : 'primary'}
          disabled={blocked || saving}
          onClick={() => void edit()}
        >
          {link ? 'Revisar vínculo' : 'Vincular CPE e AP'}
        </button>
      )}
      {!link && radio.error && !radio.busy && (
        <button className="primary" onClick={reopen}>
          Reabrir vínculo armazenado
        </button>
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
              <button
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
              </button>
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
            <button className="primary">Confirmar vínculo</button>
            <button type="button" onClick={() => setEditing(false)}>
              Cancelar
            </button>
          </fieldset>
        </form>
      )}
    </>
  );
}
function RadioLinkDeviceSelect({
  state,
  field,
  role,
  label,
}: {
  state: RadioLinkState;
  field: 'cpe_id' | 'ap_id';
  role: string;
  label: string;
}) {
  const { choice, setChoice, devices } = state;
  return (
    <label>
      {label}{' '}
      <select
        required
        aria-label={label + ' do vínculo'}
        value={choice[field]}
        onChange={(e) => setChoice({ ...choice, [field]: e.target.value })}
      >
        <option value="">Selecione</option>
        {devices
          .filter((d) => d.role === role)
          .map((d) => (
            <option key={d.id} value={d.id}>
              {d.name} · {d.address}
            </option>
          ))}
      </select>
    </label>
  );
}
function RadioManualIdentityFields({ state }: { state: RadioLinkState }) {
  const { choice, setChoice } = state;
  return (
    <>
      {[
        ['mac', 'MAC wireless da CPE'],
        ['interface', 'Interface do AP'],
        ['bssid', 'BSSID do AP'],
      ].map(([key, label]) => (
        <label key={key}>
          {label}
          <input
            required
            aria-label={label}
            maxLength={80}
            value={choice[key as 'mac' | 'interface' | 'bssid']}
            onChange={(e) => setChoice({ ...choice, [key]: e.target.value })}
          />
        </label>
      ))}
    </>
  );
}
