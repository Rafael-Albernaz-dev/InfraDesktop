import { RadioCPEClient } from './radio-cpe-client';
import type { RefreshOpeningRef } from '../equipment/use-refresh-on-open';
import { useRadioTabRefresh } from '../monitoring/automatic-tab-refresh';
import { equipmentRadioError } from './radio-equipment-status';
import { StateNotice } from '../../shared/ui/StateNotice';
import { EquipmentChain } from '../equipment/equipment-chain';
import type { Device } from '../equipment/equipment-api';
import {
  RadioEquipmentLimits,
  RadioEquipmentInterfaces,
  RadioEquipmentPagination,
} from './radio-equipment-parts';
import { createPortal } from 'react-dom';
import { Button } from '../../shared/ui/Button';
import { DataBlock } from '../../shared/ui/DataBlock';
import { RadioEquipmentView } from './radio-equipment-views';
import {
  RadioFamilies,
  RadioModes,
  RadioRoles,
} from '../equipment/radio-contract';
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
function viewOffersReopen(
  { reading, device }: RadioEquipmentState,
  tab: string,
) {
  if (!reading || (tab !== 'Clientes' && tab !== 'Rádio')) return false;
  return (
    reading.family !== RadioFamilies.unsupported ||
    device.role === RadioRoles.ap
  );
}
function RadioEquipmentMessages({
  state,
  tab,
}: {
  state: RadioEquipmentState;
  tab: string;
}) {
  const { radio, reading, credentialRefused } = state,
    page = createEquipmentPage(state),
    offerReopen =
      radio.error &&
      !radio.busy &&
      !(credentialRefused && !reading) &&
      !viewOffersReopen(state, tab);
  if (tab === 'Cliente') return null;
  return (
    <>
      {offerReopen && (
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
  tab,
}: {
  state: RadioEquipmentState;
  hidden: boolean;
  tab: string;
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
      <RadioEquipmentMessages state={state} tab={tab} />
    </>
  );
}
function RadioEquipmentBody({ state, start, tab }: RadioBodyProps) {
  const { radio } = state;
  const hidden = tab === 'Acesso' || tab === 'Roteador';
  return (
    <>
      <RadioOperationalFeedback state={state} hidden={hidden} tab={tab} />
      <div hidden={hidden}>
        {tab === 'Cliente' ? (
          <RadioCPEClient state={state} />
        ) : state.loading ? (
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
  refreshAvailable?: boolean;
  refreshDecision?: RefreshOpeningRef;
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
  useRadioTabRefresh(
    state,
    props.tab || 'Rádio',
    props.refreshAvailable === true,
    start,
    props.refreshDecision,
  );
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
