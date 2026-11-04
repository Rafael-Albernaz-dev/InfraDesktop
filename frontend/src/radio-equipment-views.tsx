import { equipmentRadioError } from './radio-equipment-status';
import type { DataState } from './ui/DataBlock';
import type { ReactNode } from 'react';
import { RadioFamilies, RadioModes } from './radio-contract';
import { useDisplayNow } from './equipment-snapshot';
import { Fragment } from 'react';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { RadioReading } from './radio-api';
import { Metrics, Reading } from './radio-reading';
import {
  formatMetric,
  formatRouterDuration,
  formatDuration,
} from './network-format';
import { DataBlock } from './ui/DataBlock';
import { DataTable } from './ui/DataTable';
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
function RadioClients({ state }: { state: Ready }) {
  const { reading } = state;
  if (!reading.clients.length) return <p>Nenhum cliente encontrado</p>;
  return (
    <DataTable>
      <thead>
        <tr>
          <th>CPE</th>
          <th>Sinal</th>
          <th>CCQ</th>
          <th>Taxas (TX/RX)</th>
          <th>Ocupação</th>
        </tr>
      </thead>
      <tbody>
        {reading.clients.map((client) => {
          const value = (name: string) => {
            const m = client.metrics.find(
              (m) => m.name === name && m.quality !== 'unavailable',
            );
            return m
              ? formatMetric(m.value, m.unit)
              : 'Não informado nesta leitura';
          };
          return (
            <tr key={client.interface + '|' + client.mac}>
              <td>
                {client.name || client.mac}{' '}
                <span className="muted">(não cadastrada)</span>
              </td>
              <td>{value('signal')}</td>
              <td>{value('ccq')}</td>
              <td>
                {value('rate_tx')} / {value('rate_rx')}
              </td>
              <td>{value('occupancy')}</td>
            </tr>
          );
        })}
      </tbody>
    </DataTable>
  );
}
function RadioInterfaces({ state }: { state: Ready }) {
  return (
    <DataTable>
      <thead>
        <tr>
          <th>Interface</th>
          <th>BSSID</th>
          <th>Canal</th>
          <th>Largura</th>
          <th>Métricas</th>
        </tr>
      </thead>
      <tbody>
        {state.reading.interfaces.map((iface) => (
          <tr key={iface.name}>
            <td>
              {iface.name}
              {iface.master && ` (AP virtual de ${iface.master})`}
            </td>
            <td>{iface.bssid}</td>
            <td>{iface.channel || 'Não informado nesta leitura'}</td>
            <td>{iface.configured_width || 'Não informado nesta leitura'}</td>
            <td>
              <Metrics metrics={iface.metrics} />
            </td>
          </tr>
        ))}
      </tbody>
    </DataTable>
  );
}
function RadioEquipmentBody({
  state,
  tab,
  limits,
  interfaces,
  pagination,
}: ViewProps) {
  return tab === 'Resumo' ? (
    <RadioIdentity state={state} />
  ) : tab === 'Clientes' ? (
    <>
      <RadioClients state={state} />
      {pagination}
    </>
  ) : tab === 'Interfaces' ? (
    <RadioInterfaces state={state} />
  ) : (
    <>
      <Metrics metrics={state.reading.metrics} />
      {tab === 'Rádio' && (
        <>
          {limits}
          {interfaces}
        </>
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
    <>
      <RadioEquipmentBody
        state={state}
        tab={tab}
        limits={limits}
        interfaces={interfaces}
        pagination={pagination}
      />
      {tab !== 'Resumo' && <RadioReadingDetails state={state} />}
    </>
  );
  return (
    <div className="grid">
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
