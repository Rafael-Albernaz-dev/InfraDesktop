import { RadioTickDiagnostic } from './radio-tick-diagnostic';
import type { RadioMetric, RadioReading } from '../equipment/radio-api';
import {
  formatDate,
  formatDuration,
  formatMacAddress,
  formatMetric,
  formatRadioFamily,
  formatRouterDuration as duration,
} from '../equipment/network-format';
import { useDisplayNow } from '../equipment/equipment-snapshot';
import { RadioFamilies } from '../equipment/radio-contract';
import { radioMessage } from './radio-messages';
const metricNames: Record<string, string> = {
  voltage: 'Tensão',
  temperature: 'Temperatura',
  signal: 'Sinal',
  remote_signal: 'Sinal informado pela CPE',
  chain0: 'Chain 0',
  chain1: 'Chain 1',
  chain2: 'Chain 2',
  chain3: 'Chain 3',
  noise: 'Ruído',
  snr: 'SNR',
  ccq: 'CCQ',
  ccq_tx: 'CCQ tx',
  ccq_rx: 'CCQ rx',
  frequency: 'Frequência',
  channel_width: 'Largura',
  rate_tx: 'Taxa PHY tx',
  rate_rx: 'Taxa PHY rx',
  distance: 'Distância',
  cpu: 'CPU da CPE',
  tx_throughput: 'Vazão tx',
  rx_throughput: 'Vazão rx',
  tx_packets_delta: 'Pacotes tx',
  rx_packets_delta: 'Pacotes rx',
  tx_frames_delta: 'Frames tx',
  rx_frames_delta: 'Frames rx',
  tx_hw_frames_delta: 'Frames rádio tx',
  rx_hw_frames_delta: 'Frames rádio rx',
  tx_retries_delta: 'Retransmissões tx',
  tx_failed_delta: 'Falhas tx',
  tx_errors_delta: 'Erros tx',
  rx_errors_delta: 'Erros rx',
};
function Basics({
  metrics,
  perspective,
}: {
  metrics: RadioMetric[];
  perspective: string;
}) {
  const expected: [string, string, string][] = [
    ['signal', 'dBm', perspective === 'AP' ? 'CPE → AP' : 'AP → CPE'],
    ['chain0', 'dBm', perspective === 'AP' ? 'CPE → AP' : 'AP → CPE'],
    ['chain1', 'dBm', perspective === 'AP' ? 'CPE → AP' : 'AP → CPE'],
    ['noise', 'dBm', 'Recepção'],
    ['snr', 'dB', 'Recepção'],
    ['ccq_tx', '%', perspective === 'AP' ? 'AP → CPE' : 'CPE → AP'],
    ['ccq_rx', '%', perspective === 'AP' ? 'CPE → AP' : 'AP → CPE'],
    ['rate_tx', 'Mbps', perspective === 'AP' ? 'AP → CPE' : 'CPE → AP'],
    ['rate_rx', 'Mbps', perspective === 'AP' ? 'CPE → AP' : 'AP → CPE'],
  ];
  const missing = expected
    .filter(
      ([name]) =>
        !metrics.some(
          (m) => m.name === name || (name === 'ccq_tx' && m.name === 'ccq'),
        ),
    )
    .map(([name, unit, direction]): RadioMetric => ({
      name,
      unit,
      direction,
      value: 0,
      perspective,
      device_id: '',
      interface: '',
      observed_at: '',
      quality: 'unavailable',
    }));
  return <Metrics metrics={[...metrics, ...missing]} />;
}

export function Metrics({ metrics }: { metrics: RadioMetric[] }) {
  const now = useDisplayNow();
  return (
    <dl className="radio-metrics">
      {metrics.map((m) => (
        <div key={m.name + '|' + m.direction + '|' + m.interface}>
          <dt>{m.label || metricNames[m.name] || m.name}</dt>
          <dd
            title={[
              m.perspective,
              m.device_id,
              m.interface,
              m.direction,
              m.unit,
              m.observed_at,
              m.quality,
            ].join(' · ')}
          >
            {m.quality === 'unavailable'
              ? 'não disponível nesta família'
              : m.quality === 'baseline'
                ? 'primeiro tick: base'
                : m.quality === 'reset'
                  ? 'reiniciado'
                  : formatMetric(
                      m.value,
                      m.unit === 'packets' ? 'pacotes' : m.unit,
                    )}{' '}
            <span className="radio-origin">
              {(
                {
                  configuration: 'Configuração',
                  receive: 'Recepção',
                  link: 'Enlace',
                } as Record<string, string>
              )[m.direction] || m.direction}
            </span>
            {m.best !== undefined && m.worst !== undefined && (
              <small>
                Melhor {formatMetric(m.best, m.unit)} · pior{' '}
                {formatMetric(m.worst, m.unit)} nesta observação
              </small>
            )}
            {m.quality === 'initial' && (
              <small>
                Leitura inicial: {formatDate(m.observed_at)} · há{' '}
                {formatDuration(
                  Math.max(0, now - Date.parse(m.observed_at)) / 1000,
                )}
              </small>
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

type ReadingProps = {
  reading?: RadioReading;
  code?: string;
  title: string;
  onlyMAC?: string;
  note?: boolean;
  standalone?: boolean;
};
type ReadingDataProps = { reading: RadioReading };
type ReadingIdentityProps = ReadingDataProps & {
  code?: string;
  age: number;
  standalone: boolean;
};

function ReadingMissing({ title, code }: { title: string; code?: string }) {
  return (
    <article className="radio-side">
      <h4>{title}</h4>
      <p>
        {code && code !== 'OK'
          ? radioMessage(code)
          : 'Nenhuma leitura disponível.'}
      </p>
    </article>
  );
}
function ReadingIdentity({
  reading,
  code,
  age,
  standalone,
}: ReadingIdentityProps) {
  const isWOM = reading.family === RadioFamilies.intelbrasWOM;
  const modelLabel =
    reading.model ||
    (isWOM
      ? 'Modelo não coletado nesta versão (sem inferir)'
      : 'Modelo não disponível');
  return (
    <>
      {code && code !== 'OK' && (
        <p role="status">
          {radioMessage(code)} A última leitura boa conserva a data abaixo.
        </p>
      )}
      <RadioTickDiagnostic failure={reading.failure} />
      {!reading.compatible && (
        <p role="status">
          Leitura de um cadastro anterior; não confirma o vínculo atual.
        </p>
      )}
      <p>
        Origem: {reading.perspective} ·{' '}
        {reading.device_name || reading.address || 'equipamento cadastrado'} ·{' '}
        {formatDate(reading.observed_at)} · há {formatDuration(age / 1000)}
      </p>
      <p>
        {reading.manufacturer || 'Fabricante não identificado'} · {modelLabel} ·{' '}
        {reading.version || 'Firmware não disponível'} ·{' '}
        {formatRadioFamily(reading.family)}
      </p>
      {standalone && (
        <p>
          Modo: {reading.mode || 'não disponível nesta família'} · Protocolo:{' '}
          {reading.wireless_protocol || 'não disponível nesta família'}
        </p>
      )}
      {isWOM && <WOMCollectionScope />}
    </>
  );
}
function WOMCollectionScope() {
  return (
    <div className="radio-wom-scope" data-testid="wom-collection-scope">
      <p>
        <strong>Coletado nesta versão (Intelbras WOM):</strong> sinal (dBm),
        chains, ruído, CCQ/qualidade, taxa PHY (Mbps), MAC/BSSID e tempo de
        enlace.
      </p>
      <p>
        <strong>Não coletado nesta versão:</strong> SSID (não coletado nesta
        versão), largura de canal (não coletado nesta versão), porta Ethernet
        (não coletado nesta versão) e DHCP/QoS (não coletado nesta versão).
      </p>
    </div>
  );
}

function ReadingLocalHealth({ reading }: ReadingDataProps) {
  return (
    <>
      {['voltage', 'temperature'].map((name) => {
        const metric = reading.metrics?.find(
          (m) => m.name === name && m.quality !== 'unavailable',
        );
        return (
          <p key={name}>
            {name === 'voltage' ? 'Tensão' : 'Temperatura'}:{' '}
            {metric
              ? formatMetric(metric.value, metric.unit)
              : 'não disponível nesta família'}
          </p>
        );
      })}
      {reading.interfaces.map((iface) => (
        <div key={iface.name}>
          <p>
            Interface {iface.name} · largura{' '}
            {iface.configured_width || 'não disponível nesta família'}
          </p>
          <Metrics metrics={iface.metrics || []} />
        </div>
      ))}
    </>
  );
}

function ReadingConnectionIdentity({ reading }: ReadingDataProps) {
  return (
    <>
      {!!reading.ssh_legacy?.length && (
        <p>SSH legado: {reading.ssh_legacy.join(', ')}</p>
      )}
      {!!reading.ssh_negotiated?.length && (
        <p>SSH negociado: {reading.ssh_negotiated.join(', ')}</p>
      )}
      {reading.ap_bssid && (
        <p>BSSID do AP: {formatMacAddress(reading.ap_bssid)}</p>
      )}
      {reading.wireless_mac && (
        <p>MAC wireless: {formatMacAddress(reading.wireless_mac)}</p>
      )}
    </>
  );
}

function ReadingQuality({ reading }: ReadingDataProps) {
  return (
    <>
      {reading.perspective === 'CPE' ? (
        <Basics metrics={reading.metrics || []} perspective="CPE" />
      ) : (
        <Metrics metrics={reading.metrics || []} />
      )}
      {reading.perspective === 'AP' &&
      reading.cpu_before_during_after?.length ? (
        <p>
          CPU do AP:{' '}
          {formatMetric(reading.cpu_before_during_after.at(-1)!, '%')}
        </p>
      ) : null}
      {reading.scope === 'link-tick' && <p>Leitura do enlace</p>}
    </>
  );
}

function ReadingUptimes({ reading }: ReadingDataProps) {
  return (
    <>
      {reading.perspective === 'CPE' && (
        <p title={reading.wireless_uptime}>
          Uptime wireless:{' '}
          {reading.wireless_uptime
            ? duration(reading.wireless_uptime)
            : 'não disponível nesta família'}
        </p>
      )}
      {reading.device_uptime && (
        <p title={reading.device_uptime}>
          Uptime do equipamento: {duration(reading.device_uptime)}
        </p>
      )}
    </>
  );
}

type ReadingClientsProps = {
  clients: RadioReading['clients'];
  perspective: string;
  standalone: boolean;
};
function ReadingClients({
  clients,
  perspective,
  standalone,
}: ReadingClientsProps) {
  return (
    <>
      {clients.map((client) => (
        <article
          key={client.interface + '|' + client.mac}
          className="radio-client"
        >
          <h5>{client.name || client.mac}</h5>
          {standalone && <p>visto pelo lado local, não cadastrado</p>}
          <p>
            {formatMacAddress(client.mac)} · {client.interface}
            {(!standalone || client.bssid !== client.mac) && (
              <> · BSSID {formatMacAddress(client.bssid)}</>
            )}
          </p>
          {!(standalone && perspective === 'CPE') && (
            <Basics metrics={client.metrics || []} perspective="AP" />
          )}
          <p title={client.wireless_uptime}>
            Uptime wireless: {duration(client.wireless_uptime)}
          </p>
          <p title={client.last_activity}>
            Visto há {duration(client.last_activity)}
          </p>
        </article>
      ))}
    </>
  );
}

export function Reading({
  reading,
  code,
  title,
  onlyMAC,
  note = true,
  standalone = false,
}: ReadingProps) {
  const now = useDisplayNow();
  if (!reading) return <ReadingMissing title={title} code={code} />;
  const age = Math.max(0, now - Date.parse(reading.observed_at));
  const clients = onlyMAC
    ? reading.clients.filter((client) => client.mac === onlyMAC)
    : reading.clients;
  return (
    <article className="radio-side">
      <h4>{title}</h4>
      <ReadingIdentity
        reading={reading}
        code={code}
        age={age}
        standalone={standalone}
      />
      {standalone && <ReadingLocalHealth reading={reading} />}
      <ReadingConnectionIdentity reading={reading} />
      <ReadingQuality reading={reading} />
      <ReadingUptimes reading={reading} />
      <ReadingClients
        clients={clients}
        perspective={reading.perspective}
        standalone={standalone}
      />
      {note && (
        <p className="radio-note">
          Taxa negociada, não é velocidade. CCQ, ruído e uptime ausentes são
          indisponíveis.
        </p>
      )}
    </article>
  );
}
