import { RadioUnsupportedNotice } from './radio-equipment-parts';
import {
  RadioClients,
  RadioInterfaces,
  radioRatePair,
} from './radio-equipment-tables';
import { KeyValues } from './ui/KeyValues';
import { Button } from './ui/Button';
import { createEquipmentPage } from './radio-equipment-state';
import { equipmentRadioError } from './radio-equipment-status';
import type { DataState } from './ui/DataBlock';
import type { ReactNode } from 'react';
import { RadioFamilies, RadioModes } from './radio-contract';
import { useDisplayNow } from './equipment-snapshot';
import { Fragment } from 'react';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { RadioReading } from './radio-api';
import { Reading } from './radio-reading';
import {
  formatMetric,
  formatRouterDuration,
  formatDuration,
} from './network-format';
import { DataBlock } from './ui/DataBlock';
import { RadioRoles } from './radio-contract';

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
    (m) => m.name === name && m.quality !== 'unavailable',
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
          title="Ao vivo do enlace"
          source="serviço local"
          data={{ state: 'unbuilt', slice: 'L-BASE-2' }}
        />
      )}
    </div>
  );
}

function RadioReadingDetails({ state }: { state: Ready }) {
  return (
    <details className="det">
      <summary>Detalhes da leitura</summary>
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
    </details>
  );
}
