import type { ReactNode } from 'react';
import { Fragment } from 'react';
import { useDisplayNow } from '../equipment/equipment-snapshot';
import {
  formatDuration,
  formatMetric,
  formatRouterDuration,
} from '../equipment/network-format';
import type { RadioReading } from '../equipment/radio-api';
import {
  RadioFamilies,
  RadioModes,
  RadioRoles,
} from '../equipment/radio-contract';
import { RadioUnsupportedNotice } from './radio-equipment-parts';
import type { RadioEquipmentState } from './radio-equipment-state';
import { createEquipmentPage } from './radio-equipment-state';
import { equipmentRadioError } from './radio-equipment-status';
import { RadioClients, RadioInterfaces } from './radio-equipment-tables';
import { RadioReadingDetails } from './radio-reading-details';
import { radioRatePair } from './radio-table-format';
import { Button } from '../../shared/ui/Button';
import type { DataState } from '../../shared/ui/DataBlock';
import { DataBlock } from '../../shared/ui/DataBlock';
import { KeyValues } from '../../shared/ui/KeyValues';

type Ready = RadioEquipmentState & { reading: RadioReading };
type ViewProps = {
  state: Ready;
  tab: string;
  limits: React.ReactNode;
  interfaces: React.ReactNode;
  pagination: React.ReactNode;
};
function metric(reading: RadioReading, name: string) {
  const value = reading.metrics.find(
    (m) =>
      (m.name === name || (name === 'ccq' && m.name === 'ccq_tx')) &&
      m.quality !== 'unavailable',
  );
  return value
    ? formatMetric(value.value, value.unit)
    : 'Não informado nesta leitura';
}
function RadioIdentity({ state }: { state: Ready }) {
  const r = state.reading;
  const rows = [
    ['Identidade', r.device_name],
    ['Modelo', r.model],
    ['Firmware', r.version],
    ['Uptime', formatRouterDuration(r.device_uptime)],
    ['CPU', metric(r, 'cpu')],
    ['Tensão', metric(r, 'voltage')],
    ['Temperatura', metric(r, 'temperature')],
  ];
  return (
    <dl className="kv">
      {rows.map(([name, value]) => (
        <Fragment key={name}>
          <dt>{name}</dt>
          <dd>{value || 'Não informado nesta leitura'}</dd>
        </Fragment>
      ))}
    </dl>
  );
}
function RadioMetricsBody({ state, tab }: { state: Ready; tab: string }) {
  const names: [string, string][] = [
    ['cpu', 'CPU'],
    ['voltage', 'Tensão'],
    ['temperature', 'Temperatura'],
    ['signal', 'Sinal'],
    ['snr', 'SNR'],
    ['noise', 'Ruído'],
    ['ccq', 'CCQ'],
  ];
  const rows: [string, ReactNode][] = names
    .filter(
      ([name]) =>
        name !== 'temperature' ||
        state.reading.metrics.some((m) => m.name === name),
    )
    .map(([name, label]) => [label, metric(state.reading, name)]);
  if (tab === 'Enlace') {
    rows.push(['Taxas (TX/RX)', radioRatePair(state.reading.metrics)]);
    const id = state.radio.view?.link?.ap_id,
      ap = state.devices?.find((d) => d.id === id);
    rows.push([
      'AP de origem',
      ap ? (
        <Button
          className="link-control"
          onClick={() => state.openEquipment?.(ap.id)}
        >
          {ap.name}
        </Button>
      ) : (
        'Não informado nesta leitura'
      ),
    ]);
  }
  return <KeyValues rows={rows} />;
}
function RadioReopen({ state }: { state: Ready }) {
  const page = createEquipmentPage(state);
  return (
    <Button onClick={() => void page(0, true)}>Reabrir rádio armazenado</Button>
  );
}
function RadioEquipmentBody({ state, tab, pagination }: ViewProps) {
  if (
    state.reading.family === RadioFamilies.unsupported &&
    state.device.role !== RadioRoles.ap
  )
    return (
      <>
        <RadioUnsupportedNotice reading={state.reading} />
        <RadioReadingDetails state={state} />
      </>
    );
  if (tab === 'Resumo') return <RadioIdentity state={state} />;
  if (tab === 'Enlace' && state.device.role === RadioRoles.ptp)
    return (
      <KeyValues
        rows={[
          ['Sinal', metric(state.reading, 'signal')],
          ['SNR', metric(state.reading, 'snr')],
        ]}
      />
    );
  if (tab === 'Clientes')
    return (
      <>
        <RadioClients state={state} />
        {pagination}
        <RadioReadingDetails state={state} />
        <div className="actions">
          <RadioReopen state={state} />
        </div>
      </>
    );
  if (tab === 'Interfaces')
    return (
      <>
        <RadioInterfaces state={state} />
        <RadioReadingDetails state={state} />
      </>
    );
  return (
    <>
      <RadioMetricsBody state={state} tab={tab} />
      {tab === 'Rádio' ? (
        <div className="actions">
          <RadioReopen state={state} />
          <RadioReadingDetails state={state} />
        </div>
      ) : (
        <RadioReadingDetails state={state} />
      )}
    </>
  );
}
function readingBlockState(state: Ready, content: ReactNode): DataState {
  if (state.radio.error)
    return {
      state: 'failed',
      reason: equipmentRadioError(state),
      technical:
        state.radio.view?.job?.code !== 'OK'
          ? state.radio.view?.job?.code || 'Falha na atualização'
          : 'Falha na atualização',
      retry: () => {
        state.setOffset(0);
        void state.radio.start(RadioModes.once, RadioFamilies.standalone);
      },
      disabled: state.blocked,
      previous: content,
    };
  return { state: state.radio.busy ? 'refreshing' : 'ready', content };
}
export function RadioEquipmentView({
  state,
  tab,
  limits,
  interfaces,
  pagination,
}: ViewProps) {
  const now = useDisplayNow();
  const content = (
    <RadioEquipmentBody
      state={state}
      tab={tab}
      limits={limits}
      interfaces={interfaces}
      pagination={pagination}
    />
  );
  return (
    <div className={['Resumo', 'Rádio', 'Enlace'].includes(tab) ? 'grid' : ''}>
      <DataBlock
        title={
          tab === 'Resumo'
            ? 'Identidade e sistema'
            : tab === 'Clientes'
              ? 'Clientes conectados'
              : state.device.role === RadioRoles.ptp && tab === 'Enlace'
                ? 'Lado A'
                : tab
        }
        source="SSH"
        age={
          'há ' +
          formatDuration((now - Date.parse(state.reading.observed_at)) / 1000)
        }
        data={readingBlockState(state, content)}
      />
      {tab === 'Rádio' && (
        <>
          {interfaces}
          {limits}
        </>
      )}
      {tab === 'Enlace' && (
        <DataBlock
          title={
            state.device.role === RadioRoles.ptp
              ? 'Lado B'
              : 'Ao vivo do enlace'
          }
          source="serviço local"
          data={{ state: 'unbuilt', slice: 'L07-DRV' }}
        />
      )}
    </div>
  );
}
