import {
  RadioCodes,
  RadioFamilies,
  RadioModes,
  RadioRoles,
} from './radio-contract';
import type { RadioInterface, RadioReading } from './radio-api';
import { exactNumber, formatRadioFamily } from './network-format';
import { radioMessage } from './radio-messages';
import { Reading, Metrics } from './radio-reading';
import { RadioCredentialRecovery } from './radio-credential-recovery';
import {
  createEquipmentPage,
  createFamilyConfirmation,
  useCompletedEquipmentRefresh,
  useRadioEquipmentState,
  useStoredEquipmentRadio,
} from './radio-equipment-state';
import type {
  RadioEquipmentProps,
  RadioEquipmentState,
} from './radio-equipment-state';

type ReadyEquipment = RadioEquipmentState & { reading: RadioReading };

function RadioEquipmentActions({ state }: { state: RadioEquipmentState }) {
  const {
    radio,
    reading,
    blocked,
    setOffset,
    profile,
    activation,
    device,
    setCurrentDevice,
    credentialRefused,
  } = state;
  return (
    <>
      {radio.busy ? (
        <button className="primary" onClick={() => void radio.stop()}>
          Parar
        </button>
      ) : (
        <button
          className={radio.error && !reading ? '' : 'primary'}
          disabled={blocked}
          onClick={() => {
            setOffset(0);
            void radio.start(RadioModes.once, RadioFamilies.standalone);
          }}
        >
          Consultar rádio
        </button>
      )}
      {radio.error && (
        <p role="alert">
          {credentialRefused
            ? radioMessage(RadioCodes.REMOTE_AUTH_FAILED)
            : radio.error}
        </p>
      )}
      {credentialRefused && (
        <RadioCredentialRecovery
          profile={profile}
          activation={activation}
          deviceID={device.id}
          initialMessage=""
          observed={(next) =>
            setCurrentDevice((current) =>
              next.revision >= current.revision ? next : current,
            )
          }
          changed={(next) => {
            setCurrentDevice(next);
            radio.setError('');
            radio.setView(undefined);
          }}
        />
      )}
    </>
  );
}

function RadioEquipmentMessages({ state }: { state: RadioEquipmentState }) {
  const { radio, reading, credentialRefused } = state,
    page = createEquipmentPage(state);
  return (
    <>
      {radio.error && !radio.busy && !(credentialRefused && !reading) && (
        <button
          className={!reading ? 'primary' : ''}
          onClick={() => void page(0, true)}
        >
          Reabrir rádio armazenado
        </button>
      )}
      {!reading && !radio.busy && !radio.error && (
        <p>Nenhuma leitura de rádio salva. Consulte o destino cadastrado.</p>
      )}
    </>
  );
}

function RadioThresholdFields({ state }: { state: RadioEquipmentState }) {
  const { warning, critical, setWarning, setCritical } = state;
  return (
    <>
      <label>
        Aviso a partir de{' '}
        <input
          aria-label="Aviso de painel lotado"
          type="number"
          min="1"
          max="511"
          value={warning}
          onChange={(event) => setWarning(Number(event.target.value))}
        />
      </label>
      <label>
        Crítico a partir de{' '}
        <input
          aria-label="Crítico de painel lotado"
          type="number"
          min="2"
          max="512"
          value={critical}
          onChange={(event) => setCritical(Number(event.target.value))}
        />
      </label>
      <p>Limiares configuráveis · origem: herdado do Atlas.</p>
    </>
  );
}

function RadioEquipmentLimits({ state }: { state: ReadyEquipment }) {
  const { reading, device, radio, blocked, warning, critical, limits } = state;
  const confirm = createFamilyConfirmation(state);
  if (reading.family === RadioFamilies.unsupported)
    return (
      <p className="radio-note">
        Driver ausente ({reading.model || 'modelo não identificado'} /{' '}
        {reading.version || 'firmware não identificado'}).{' '}
        {reading.driver_reason || 'Família ou versão sem driver comprovado.'}
      </p>
    );
  if (device.role !== RadioRoles.ap) return null;
  return (
    <fieldset disabled={radio.busy || blocked}>
      <legend>Família e painel lotado</legend>
      <p>
        Detectada: {formatRadioFamily(reading.family)} ·{' '}
        {limits?.confirmed ? 'confirmada' : 'aguarda confirmação'}
      </p>
      {device.role === RadioRoles.ap && <RadioThresholdFields state={state} />}
      <button
        disabled={
          warning < 1 ||
          critical <= warning ||
          critical > 512 ||
          !reading.compatible
        }
        onClick={() => void confirm()}
      >
        {limits?.confirmed ? 'Salvar limites' : 'Confirmar família detectada'}
      </button>
    </fieldset>
  );
}

type InterfaceProps = {
  iface: RadioInterface;
  warning: number;
  critical: number;
};
function RadioInterfaceOccupancy({ iface, warning, critical }: InterfaceProps) {
  return (
    <>
      <p>
        Clientes:{' '}
        {iface.clients === undefined
          ? 'Indisponível'
          : exactNumber(iface.clients)}
      </p>
      {iface.clients !== undefined && iface.clients >= critical ? (
        <p role="status">Painel lotado · crítico</p>
      ) : iface.clients !== undefined && iface.clients >= warning ? (
        <p role="status">Painel lotado · aviso</p>
      ) : null}
      {iface.clients === 0 && iface.had_clients && (
        <p role="status">Clientes zero: este AP já teve clientes.</p>
      )}
    </>
  );
}

function RadioInterfaceView({ iface, warning, critical }: InterfaceProps) {
  return (
    <div className={iface.master ? 'radio-virtual' : ''}>
      <h5>
        {iface.master ? 'AP virtual · ' : ''}
        {iface.name}
      </h5>
      <p>
        BSSID {iface.bssid} · {iface.band || 'Banda indisponível'}
      </p>
      <p>
        Canal observado: {iface.channel || 'Indisponível'} · largura
        configurada: {iface.configured_width || 'herdada da master'}
      </p>
      {iface.power_metrics?.length ? (
        <details>
          <summary>Potência por taxa</summary>
          <Metrics metrics={iface.power_metrics} />
        </details>
      ) : (
        <p>Potência indisponível</p>
      )}
      <RadioInterfaceOccupancy
        iface={iface}
        warning={warning}
        critical={critical}
      />
      <Metrics metrics={iface.metrics || []} />
    </div>
  );
}

function RadioEquipmentInterfaces({ state }: { state: ReadyEquipment }) {
  const { device, reading, warning, critical } = state;
  if (device.role !== RadioRoles.ap) return null;
  return (
    <div className="radio-interfaces">
      {reading.interfaces
        .filter((iface) => !iface.master)
        .map((master) => (
          <article key={master.name}>
            <h4>
              {master.name} · {master.bssid}
            </h4>
            {[
              master,
              ...reading.interfaces.filter(
                (iface) => iface.master === master.name,
              ),
            ].map((iface) => (
              <RadioInterfaceView
                key={iface.name}
                iface={iface}
                warning={warning}
                critical={critical}
              />
            ))}
          </article>
        ))}
    </div>
  );
}

function RadioEquipmentPagination({ state }: { state: ReadyEquipment }) {
  const { offset, radio } = state,
    page = createEquipmentPage(state);
  if (!(offset > 0 || radio.view?.more)) return null;
  return (
    <nav aria-label="Clientes do rádio">
      {offset > 0 && (
        <button
          disabled={radio.busy}
          onClick={() => void page(Math.max(0, offset - 8))}
        >
          Clientes anteriores
        </button>
      )}
      {radio.view?.more && (
        <button
          disabled={radio.busy}
          onClick={() => void page(radio.view?.next_offset || 0)}
        >
          Próximos clientes
        </button>
      )}
    </nav>
  );
}

function RadioEquipmentReading({ state }: { state: ReadyEquipment }) {
  return (
    <>
      <RadioEquipmentLimits state={state} />
      <RadioEquipmentInterfaces state={state} />
      <Reading
        reading={state.reading}
        standalone
        code={state.radio.view?.ap_code}
        title={
          state.device.role === RadioRoles.ap
            ? 'Clientes do AP'
            : 'Perspectiva da CPE'
        }
      />
      <RadioEquipmentPagination state={state} />
    </>
  );
}

export function RadioEquipment(props: RadioEquipmentProps) {
  const { rootRef, ...state } = useRadioEquipmentState(props);
  useStoredEquipmentRadio(state);
  useCompletedEquipmentRefresh(state);
  return (
    <section
      className="radio-panel"
      ref={rootRef}
      aria-label="Rádio do equipamento"
    >
      <h3>Rádio do equipamento</h3>
      {state.loading ? (
        <p role="status">Lendo rádio armazenado…</p>
      ) : (
        <>
          <RadioEquipmentActions state={state} />
          <RadioEquipmentMessages state={state} />
          {state.reading && (
            <RadioEquipmentReading
              state={{ ...state, reading: state.reading }}
            />
          )}
        </>
      )}
    </section>
  );
}
