import { equipmentRadioError } from './radio-equipment-status';
import { StateNotice } from './ui/StateNotice';
import { EquipmentChain } from './equipment-chain';
import type { Device } from './equipment-api';
import {
  RadioEquipmentLimits,
  RadioEquipmentInterfaces,
  RadioEquipmentPagination,
} from './radio-equipment-parts';
import { createPortal } from 'react-dom';
import { Button } from './ui/Button';
import { DataBlock } from './ui/DataBlock';
import { RadioEquipmentView } from './radio-equipment-views';
import { RadioFamilies, RadioModes } from './radio-contract';
import type { RadioReading } from './radio-api';
import { RadioCredentialRecovery } from './radio-credential-recovery';
import {
  createEquipmentPage,
  useCompletedEquipmentRefresh,
  useRadioEquipmentState,
  useStoredEquipmentRadio,
} from './radio-equipment-state';
import type {
  RadioEquipmentProps,
  RadioEquipmentState,
} from './radio-equipment-state';

type ReadyEquipment = RadioEquipmentState & { reading: RadioReading };

function RadioEquipmentMessages({ state }: { state: RadioEquipmentState }) {
  const { radio, reading, credentialRefused } = state,
    page = createEquipmentPage(state);
  return (
    <>
      {radio.error && !radio.busy && !(credentialRefused && !reading) && (
        <Button
          type="submit"
          className={!reading ? 'primary' : ''}
          onClick={() => void page(0, true)}
        >
          Reabrir rádio armazenado
        </Button>
      )}
      {!reading && !radio.busy && !radio.error && (
        <p>Nenhuma leitura de rádio salva. Consulte o destino cadastrado.</p>
      )}
    </>
  );
}

function RadioEquipmentReading({
  state,
  tab,
}: {
  state: ReadyEquipment;
  tab: string;
}) {
  return (
    <RadioEquipmentView
      state={state}
      tab={tab}
      limits={<RadioEquipmentLimits state={state} />}
      interfaces={<RadioEquipmentInterfaces state={state} />}
      pagination={<RadioEquipmentPagination state={state} />}
    />
  );
}

type RadioBodyProps = {
  state: RadioEquipmentState;
  start: () => void;
  tab: string;
};
function RadioOperationalFeedback({
  state,
  hidden,
}: {
  state: RadioEquipmentState;
  hidden: boolean;
}) {
  const { radio } = state;
  return (
    <>
      {state.credentialRefused && (
        <RadioCredentialRecovery
          profile={state.profile}
          activation={state.activation}
          deviceID={state.device.id}
          initialMessage=""
          observed={(next) =>
            state.setCurrentDevice((current) =>
              next.revision >= current.revision ? next : current,
            )
          }
          changed={(next) => {
            state.setCurrentDevice(next);
            radio.setError('');
            radio.setView(undefined);
          }}
        />
      )}
      {hidden && radio.error && (
        <StateNotice error>{equipmentRadioError(state)}</StateNotice>
      )}
      <RadioEquipmentMessages state={state} />
    </>
  );
}
function RadioEquipmentBody({ state, start, tab }: RadioBodyProps) {
  const { radio } = state;
  const hidden = tab === 'Acesso' || tab === 'Cliente' || tab === 'Roteador';
  return (
    <>
      <RadioOperationalFeedback state={state} hidden={hidden} />
      <div hidden={hidden}>
        {state.loading ? (
          <DataBlock title="Rádio" source="SSH" data={{ state: 'loading' }} />
        ) : state.reading ? (
          <RadioEquipmentReading
            state={{ ...state, reading: state.reading }}
            tab={tab}
          />
        ) : (
          <DataBlock
            title="Identidade e sistema"
            source="SSH"
            data={
              radio.error
                ? {
                    state: 'failed',
                    reason: radio.error,
                    technical: radio.view?.job?.code || 'Falha da leitura',
                    retry: start,
                    disabled: state.blocked,
                  }
                : { state: 'unread', action: start, disabled: state.blocked }
            }
          />
        )}
      </div>
    </>
  );
}

type RadioEquipmentViewProps = RadioEquipmentProps & {
  tab?: string;
  actionHost?: HTMLElement | null;
  chainHost?: HTMLElement | null;
  devices?: Device[];
  openEquipment?: (id: string) => void;
};
function RadioPrimaryAction({
  state,
  start,
}: {
  state: RadioEquipmentState;
  start: () => void;
}) {
  return state.radio.busy ? (
    <Button kind="primary" onClick={() => void state.radio.stop()}>
      Parar
    </Button>
  ) : (
    <Button kind="primary" disabled={state.blocked} onClick={start}>
      {state.device.fingerprint ? 'Atualizar' : 'Conectar'}
    </Button>
  );
}
export function RadioEquipment(props: RadioEquipmentViewProps) {
  const { rootRef, ...state } = useRadioEquipmentState(props);
  useStoredEquipmentRadio(state);
  useCompletedEquipmentRefresh(state);
  const { radio } = state;
  const start = () => {
    state.setOffset(0);
    void radio.start(RadioModes.once, RadioFamilies.standalone);
  };
  const primary = <RadioPrimaryAction state={state} start={start} />;
  return (
    <section
      ref={(node) => {
        rootRef.current =
          node?.closest<HTMLElement>('.equipment-sheet') || node;
      }}
      aria-label="Rádio do equipamento"
    >
      {props.chainHost &&
        createPortal(
          <EquipmentChain
            device={props.device}
            devices={props.devices || []}
            link={radio.view?.link}
            open={props.openEquipment || (() => {})}
          />,
          props.chainHost,
        )}
      {props.actionHost ? createPortal(primary, props.actionHost) : primary}
      <RadioEquipmentBody
        state={state}
        start={start}
        tab={props.tab || 'Rádio'}
      />
    </section>
  );
}
