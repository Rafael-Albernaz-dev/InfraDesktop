import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { radioAPI as api } from './radio-api';
import type {
  RadioInput,
  RadioLinkChoice,
  RadioMetric,
  RadioReading,
  RadioResult,
  RadioView,
} from './radio-api';
import type { Device } from './equipment-api';
import { equipmentAPI, equipmentMessage } from './equipment-api';
import {
  formatDate,
  exactNumber,
  formatDuration,
  formatMetric,
  formatRouterDuration as duration,
} from './network-format';
import { useDisplayNow } from './equipment-snapshot';
import './radio.css';

const families = [
  ['routeros-wireless-v6', 'MikroTik wireless v6'],
  ['airos-v6', 'Ubiquiti airOS v6'],
  ['intelbras-wom', 'Intelbras WOM'],
];
const radioMessage = (code: string) =>
  ({
    RADIO_SESSION_OFFLINE: 'Enlace indisponível: sessão OFFLINE.',
    RADIO_CPE_UNREACHABLE:
      'A CPE cadastrada não corresponde ao IP desta sessão. Confira o vínculo e o caminho de gestão.',
    RADIO_LINK_REQUIRED:
      'Confirme o vínculo com uma CPE e um AP cadastrados antes de consultar.',
    RADIO_LINK_UNCONFIRMED:
      'Vínculo não confirmado agora. Confira MAC, AP e interface; o vínculo salvo foi preservado.',
    RADIO_FAMILY_CONFIRMATION_REQUIRED:
      'Abra o equipamento, consulte e confirme a família detectada.',
    RADIO_LIVE_COST_REQUIRED: 'Meça o custo do enlace antes de iniciar o live.',
    RADIO_LIVE_ABORTED:
      'Live interrompido após duas falhas ou ultrapassagens consecutivas de duração, CPU ou ping.',
    RADIO_WINDOW_ENDED:
      'A janela do live terminou. Inicie outra observação deliberadamente.',
    RADIO_LOAD_UNAVAILABLE:
      'Medição de CPU e ping indisponível; live desligado.',
    OBSERVATION_UNAVAILABLE:
      'Os dados foram atualizados. Reabra os dados armazenados neste contexto.',
    UNSUPPORTED:
      'Este driver não identifica um rádio suportado neste firmware.',
  })[code] || equipmentMessage(code);

const metricNames: Record<string, string> = {
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
  rate_tx: 'Taxa tx',
  rate_rx: 'Taxa rx',
  distance: 'Distância',
};
function Metrics({ metrics }: { metrics: RadioMetric[] }) {
  return (
    <dl className="radio-metrics">
      {metrics.map((m) => (
        <div key={m.name + '|' + m.direction + '|' + m.interface}>
          <dt>{m.label || metricNames[m.name] || m.name}</dt>
          <dd
            title={`${m.perspective} · ${m.device_id} · ${m.interface} · ${m.direction} · ${m.unit} · ${m.observed_at} · ${m.quality}`}
          >
            {formatMetric(m.value, m.unit)}{' '}
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
          </dd>
        </div>
      ))}
    </dl>
  );
}
function Reading({
  reading,
  code,
  title,
  onlyMAC,
  note = true,
}: {
  reading?: RadioReading;
  code?: string;
  title: string;
  onlyMAC?: string;
  note?: boolean;
}) {
  const now = useDisplayNow();
  if (!reading)
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
  const age = Math.max(0, now - Date.parse(reading.observed_at));
  const clients = onlyMAC
    ? reading.clients.filter((c) => c.mac === onlyMAC)
    : reading.clients;
  return (
    <article className="radio-side">
      <h4>{title}</h4>
      {code && code !== 'OK' && (
        <p role="status">
          {radioMessage(code)} A última leitura boa conserva a data abaixo.
        </p>
      )}
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
        {reading.version} · {reading.family}
      </p>
      {!!reading.ssh_legacy?.length && (
        <p>SSH legado: {reading.ssh_legacy.join(', ')}</p>
      )}
      {reading.ap_bssid && <p>BSSID do AP: {reading.ap_bssid}</p>}
      {reading.wireless_mac && <p>MAC wireless: {reading.wireless_mac}</p>}
      <Metrics metrics={reading.metrics || []} />
      {reading.perspective === 'CPE' && (
        <p title={reading.wireless_uptime}>
          Uptime wireless: {duration(reading.wireless_uptime)}
        </p>
      )}
      {reading.device_uptime && (
        <p title={reading.device_uptime}>
          Uptime do equipamento: {duration(reading.device_uptime)}
        </p>
      )}
      {clients.map((c) => (
        <article key={c.interface + '|' + c.mac} className="radio-client">
          <h5>{c.name || c.mac}</h5>
          <p>
            {c.mac} · {c.interface} · BSSID {c.bssid}
          </p>
          <Metrics metrics={c.metrics || []} />
          <p title={c.wireless_uptime}>
            Uptime wireless: {duration(c.wireless_uptime)}
          </p>
          <p title={c.last_activity}>Visto há {duration(c.last_activity)}</p>
        </article>
      ))}
      {note && (
        <p className="radio-note">
          Taxa negociada, não é velocidade. CCQ, ruído e uptime ausentes são
          indisponíveis.
        </p>
      )}
    </article>
  );
}

function useRadio(
  profile: string,
  activation: string,
  device: Device,
  observation = '',
  row = '',
) {
  const [view, setView] = useState<RadioView>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [jobID, setJobID] = useState('');
  const life = useRef({ generation: 0 }),
    operation = useRef(''),
    job = useRef(''),
    stopping = useRef(false);
  const root = useRef<HTMLElement>(null);
  const identity = [
    profile,
    activation,
    device.id,
    device.revision,
    observation,
    row,
  ].join('|');
  useLayoutEffect(() => {
    const lifecycle = life.current;
    lifecycle.generation++;
    return () => {
      lifecycle.generation++;
    };
  }, [identity]);
  useEffect(() => {
    return () => {
      if (operation.current)
        void api
          .cancelOperation(profile, activation, device.id, operation.current)
          .catch(() => {});
    };
  }, [profile, activation, device.id]);
  const accept = (r: RadioResult, g: number) =>
    g === life.current.generation &&
    r.profile_id === profile &&
    r.activation_id === activation;
  async function stop() {
    if (stopping.current) return;
    stopping.current = true;
    const g = life.current.generation;
    try {
      const r = job.current
        ? await api.cancel(profile, activation, job.current)
        : operation.current
          ? await api.cancelOperation(
              profile,
              activation,
              device.id,
              operation.current,
            )
          : undefined;
      if (g !== life.current.generation) return;
      if (r && r.code !== 'OK') {
        setError(radioMessage(r.code));
        return;
      }
      operation.current = '';
      job.current = '';
      setJobID('');
      setBusy(false);
    } catch {
      if (g === life.current.generation)
        setError(
          'Parada incerta. Confira o estado da ação antes de iniciar outra.',
        );
    } finally {
      stopping.current = false;
    }
  }
  const stopRef = useRef(stop);
  useLayoutEffect(() => {
    stopRef.current = stop;
  });
  useEffect(() => {
    const ownedDocument = root.current?.ownerDocument;
    if (!ownedDocument) return;
    const hidden = () => {
      if (ownedDocument.visibilityState !== 'visible' && operation.current)
        void stopRef.current();
    };
    ownedDocument.addEventListener('visibilitychange', hidden);
    const observer =
      typeof IntersectionObserver === 'undefined'
        ? undefined
        : new IntersectionObserver((entries) => {
            if (entries.some((e) => !e.isIntersecting) && operation.current)
              void stopRef.current();
          });
    if (root.current) observer?.observe(root.current);
    return () => {
      ownedDocument.removeEventListener('visibilitychange', hidden);
      observer?.disconnect();
    };
  }, []);
  useEffect(() => {
    if (!busy || !jobID) return;
    const g = life.current.generation;
    const pollingOperation = operation.current;
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const r = await api.status(profile, activation, jobID);
        if (
          !alive ||
          !accept(r, g) ||
          stopping.current ||
          operation.current !== pollingOperation
        )
          return;
        if (r.code !== 'OK') {
          setError(radioMessage(r.code));
          return;
        }
        if (r.data) {
          setView((old) => ({
            total: 0,
            more: false,
            next_offset: 0,
            ...old,
            ...r.data,
          }));
        }
        if (r.data?.job?.state === 'RUNNING')
          timer = setTimeout(() => void poll(), 200);
        else {
          operation.current = '';
          job.current = '';
          setJobID('');
          setBusy(false);
          if (
            r.data?.job?.code &&
            !['OK', 'CANCELLED'].includes(r.data.job.code)
          )
            setError(radioMessage(r.data.job.code));
        }
      } catch {
        if (alive && g === life.current.generation)
          setError(
            'Não foi possível consultar o estado. Use Parar para encerrar a ação; nenhuma leitura foi reenviada.',
          );
      }
    };
    void poll();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
    // A job is polled only after its explicit start. Context changes unmount this surface.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [busy, jobID, profile, activation, identity]);
  async function start(mode: string, family?: string) {
    if (operation.current || stopping.current) return;
    const g = life.current.generation,
      o = crypto.randomUUID();
    operation.current = o;
    setError('');
    setBusy(true);
    const input: RadioInput = {
      device_id: device.id,
      revision: device.revision,
      operation_id: o,
      mode,
      ...(family ? { family } : {}),
      ...(row ? { observation_id: observation, row_id: row } : {}),
    };
    try {
      const r = await api.start(profile, activation, input);
      if (!accept(r, g) || stopping.current || operation.current !== o) {
        void api
          .cancelOperation(profile, activation, device.id, o)
          .catch(() => {});
        return;
      }
      if (r.code !== 'OK' || !r.data?.job) {
        operation.current = '';
        setBusy(false);
        setError(radioMessage(r.code));
        return;
      }
      job.current = r.data.job.id;
      setJobID(r.data.job.id);
      setView((old) => ({
        total: 0,
        more: false,
        next_offset: 0,
        ...old,
        ...r.data,
      }));
    } catch {
      if (
        g === life.current.generation &&
        operation.current === o &&
        !stopping.current
      )
        setError(
          'Resposta incerta. Use Parar para encerrar a ação antes de iniciar outra.',
        );
    }
  }
  return { view, setView, error, setError, busy, start, stop, root };
}

export function RadioEquipment({
  profile,
  activation,
  device,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  blocked: boolean;
}) {
  const { root: rootRef, ...radio } = useRadio(profile, activation, device);
  const [family, setFamily] = useState(
    device.role === 'ap' ? 'routeros-wireless-v6' : '',
  );
  const [loading, setLoading] = useState(true),
    [offset, setOffset] = useState(0);
  const [warning, setWarning] = useState(15),
    [critical, setCritical] = useState(20);
  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const r = await api.current(profile, activation, device.id);
        if (!alive) return;
        if (r.code !== 'OK') {
          radio.setError(radioMessage(r.code));
          return;
        }
        radio.setView(r.data);
        if (r.data?.profile) {
          setWarning(r.data.profile.warning);
          setCritical(r.data.profile.critical);
          if (r.data.profile.family !== 'unsupported')
            setFamily(r.data.profile.family);
        }
      } catch {
        if (alive) radio.setError('Não foi possível abrir a leitura salva.');
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile, activation, device.id]);
  async function confirm() {
    try {
      const r = await api.confirmFamily(
        profile,
        activation,
        device.id,
        radio.view?.reading?.family || family,
        device.revision,
        warning,
        critical,
      );
      if (r.code !== 'OK') {
        radio.setError(radioMessage(r.code));
        return;
      }
      radio.setView(r.data);
      radio.setError('');
    } catch {
      radio.setError(
        'Confirmação incerta. Reabra os dados salvos antes de repetir.',
      );
    }
  }
  async function page(next: number, fresh = false) {
    try {
      const r = await api.current(
        profile,
        activation,
        device.id,
        fresh ? '' : radio.view?.reading?.id || '',
        next,
      );
      if (r.code !== 'OK') {
        radio.setError(radioMessage(r.code));
        return;
      }
      radio.setView(r.data);
      radio.setError('');
      setOffset(next);
    } catch {
      radio.setError('Página indisponível. Reabra a leitura salva.');
    }
  }
  const reading = radio.view?.reading,
    p = radio.view?.profile;
  return (
    <section
      className="radio-panel"
      ref={rootRef}
      aria-label="Rádio do equipamento"
    >
      <h3>Rádio do equipamento</h3>
      {loading ? (
        <p role="status">Lendo rádio armazenado…</p>
      ) : (
        <>
          {!radio.busy && (
            <label>
              Família esperada{' '}
              <select
                aria-label="Família do rádio"
                value={family}
                onChange={(e) => setFamily(e.target.value)}
              >
                <option value="">Selecione a família</option>
                {families
                  .filter(([f]) =>
                    device.role === 'ap'
                      ? f === 'routeros-wireless-v6'
                      : f !== 'routeros-wireless-v6',
                  )
                  .map(([v, n]) => (
                    <option key={v} value={v}>
                      {n}
                    </option>
                  ))}
              </select>
            </label>
          )}
          {radio.busy ? (
            <button className="primary" onClick={() => void radio.stop()}>
              Parar
            </button>
          ) : (
            <button
              className={radio.error && !reading ? '' : 'primary'}
              disabled={blocked || !family}
              onClick={() => {
                setOffset(0);
                void radio.start('once', family);
              }}
            >
              Consultar rádio
            </button>
          )}
          {radio.error && <p role="alert">{radio.error}</p>}
          {radio.error && !radio.busy && (
            <button
              className={!reading ? 'primary' : ''}
              onClick={() => void page(0, true)}
            >
              Reabrir rádio armazenado
            </button>
          )}
          {!reading && !radio.busy && !radio.error && (
            <p>
              Nenhuma leitura de rádio salva. Escolha a família e consulte o
              destino cadastrado.
            </p>
          )}
          {reading && (
            <>
              <fieldset disabled={radio.busy || blocked}>
                <legend>Família e painel lotado</legend>
                <p>
                  Detectada: {reading.family} ·{' '}
                  {p?.confirmed ? 'confirmada' : 'aguarda confirmação'}
                </p>
                {device.role === 'ap' && (
                  <>
                    <label>
                      Aviso a partir de{' '}
                      <input
                        aria-label="Aviso de painel lotado"
                        type="number"
                        min="1"
                        max="511"
                        value={warning}
                        onChange={(e) => setWarning(Number(e.target.value))}
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
                        onChange={(e) => setCritical(Number(e.target.value))}
                      />
                    </label>
                    <p>Limiares configuráveis · origem: herdado do Atlas.</p>
                  </>
                )}
                <button
                  disabled={
                    warning < 1 ||
                    critical <= warning ||
                    critical > 512 ||
                    !reading.compatible
                  }
                  onClick={() => void confirm()}
                >
                  {p?.confirmed
                    ? 'Salvar limites'
                    : 'Confirmar família detectada'}
                </button>
              </fieldset>
              {device.role === 'ap' && (
                <div className="radio-interfaces">
                  {reading.interfaces
                    .filter((i) => !i.master)
                    .map((master) => (
                      <article key={master.name}>
                        <h4>
                          {master.name} · {master.bssid}
                        </h4>
                        {[
                          master,
                          ...reading.interfaces.filter(
                            (i) => i.master === master.name,
                          ),
                        ].map((i) => (
                          <div
                            className={i.master ? 'radio-virtual' : ''}
                            key={i.name}
                          >
                            <h5>
                              {i.master ? 'AP virtual · ' : ''}
                              {i.name}
                            </h5>
                            <p>
                              BSSID {i.bssid} · {i.band || 'Banda indisponível'}
                            </p>
                            <p>
                              Canal observado: {i.channel || 'Indisponível'} ·
                              largura configurada:{' '}
                              {i.configured_width || 'herdada da master'}
                            </p>
                            {i.power_metrics?.length ? (
                              <details>
                                <summary>Potência por taxa</summary>
                                <Metrics metrics={i.power_metrics} />
                              </details>
                            ) : (
                              <p>Potência indisponível</p>
                            )}
                            <p>
                              Clientes:{' '}
                              {i.clients === undefined
                                ? 'Indisponível'
                                : exactNumber(i.clients)}
                            </p>
                            {i.clients !== undefined &&
                            i.clients >= critical ? (
                              <p role="status">Painel lotado · crítico</p>
                            ) : i.clients !== undefined &&
                              i.clients >= warning ? (
                              <p role="status">Painel lotado · aviso</p>
                            ) : null}
                            {i.clients === 0 && i.had_clients && (
                              <p role="status">
                                Clientes zero: este AP já teve clientes.
                              </p>
                            )}
                            <Metrics metrics={i.metrics || []} />
                          </div>
                        ))}
                      </article>
                    ))}
                </div>
              )}
              <Reading
                reading={reading}
                code={radio.view?.ap_code}
                title={
                  device.role === 'ap' ? 'Clientes do AP' : 'Perspectiva da CPE'
                }
              />
              {(offset > 0 || radio.view?.more) && (
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
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}

export function RadioLinkPanel({
  profile,
  activation,
  device,
  observation,
  row,
  pppoeUptime,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  pppoeUptime: string;
  blocked: boolean;
}) {
  const { root: rootRef, ...radio } = useRadio(
    profile,
    activation,
    device,
    observation,
    row,
  );
  const [loading, setLoading] = useState(true),
    [devices, setDevices] = useState<Device[]>([]);
  const [reload, setReload] = useState(0);
  const [choice, setChoice] = useState<RadioLinkChoice>({
    ap_id: '',
    cpe_id: '',
    mac: '',
    interface: '',
    bssid: '',
    evidence: 'Confirmação manual do técnico',
    origin: 'manual',
  });
  const [editing, setEditing] = useState(false),
    [saving, setSaving] = useState(false);
  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const r = await api.link(
          profile,
          activation,
          device.id,
          observation,
          row,
        );
        if (!alive) return;
        if (r.code !== 'OK') {
          radio.setError(radioMessage(r.code));
          return;
        }
        radio.setView(r.data);
      } catch {
        if (alive) radio.setError('Não foi possível abrir o vínculo salvo.');
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile, activation, device.id, observation, row, reload]);
  async function edit() {
    setEditing(true);
    try {
      const all: Device[] = [];
      let cursor = 0;
      for (let page = 0; page < 16; page++) {
        const r = await equipmentAPI.list(
          profile,
          activation,
          '',
          cursor,
          false,
        );
        if (r.code !== 'OK') throw new Error('inventory');
        all.push(...(r.data?.devices || []));
        if (!r.data?.next_cursor) break;
        cursor = r.data.next_cursor;
      }
      setDevices(all.filter((d) => d.role === 'ap' || d.role === 'cpe'));
    } catch {
      radio.setError('Inventário indisponível para confirmar o vínculo.');
    }
  }
  async function confirm(selected: RadioLinkChoice) {
    if (saving) return;
    setSaving(true);
    radio.setError('');
    try {
      const r = await api.confirmLink(
        profile,
        activation,
        device.id,
        observation,
        row,
        crypto.randomUUID(),
        selected,
      );
      if (r.code !== 'OK') {
        radio.setError(radioMessage(r.code));
        return;
      }
      radio.setView(r.data);
      setEditing(false);
    } catch {
      radio.setError(
        'Confirmação incerta. Reabra o detalhe para conferir o vínculo salvo.',
      );
    } finally {
      setSaving(false);
    }
  }
  const link = radio.view?.link,
    cost = radio.view?.cost;
  return (
    <section className="radio-panel" ref={rootRef} aria-label="Enlace rádio">
      <h3>Enlace rádio</h3>
      <p>Uptime PPPoE: {pppoeUptime}</p>
      {loading ? (
        <p role="status">Lendo vínculo armazenado…</p>
      ) : (
        <>
          {radio.error && <p role="alert">{radio.error}</p>}
          {link ? (
            <>
              <p>
                Vínculo confirmado em {formatDate(link.confirmed_at)} ·{' '}
                {link.origin}
              </p>
              <p>
                {link.mac} · {link.interface} · BSSID {link.bssid}
              </p>
              {!link.confirmed_now && (
                <p role="status">
                  Vínculo não confirmado agora. O cadastro foi preservado.
                </p>
              )}
            </>
          ) : (
            !radio.error && <p>Nenhum vínculo confirmado para esta sessão.</p>
          )}
          {!radio.busy && !editing && !radio.error && (
            <button
              className={link ? '' : 'primary'}
              disabled={blocked || saving}
              onClick={() => void edit()}
            >
              {link ? 'Revisar vínculo' : 'Vincular CPE e AP'}
            </button>
          )}
          {!link && radio.error && !radio.busy && (
            <button
              className="primary"
              onClick={() => {
                radio.setError('');
                setLoading(true);
                setReload((n) => n + 1);
              }}
            >
              Reabrir vínculo armazenado
            </button>
          )}
          {!link &&
            !editing &&
            !radio.busy &&
            !!radio.view?.suggestions?.length && (
              <div>
                <p>
                  Sugestões pela registration table; confirme a identidade antes
                  de usar.
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
          {editing && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void confirm(choice);
              }}
            >
              <fieldset disabled={saving || blocked}>
                <legend>Confirmação do vínculo</legend>
                <label>
                  CPE{' '}
                  <select
                    required
                    aria-label="CPE do vínculo"
                    value={choice.cpe_id}
                    onChange={(e) =>
                      setChoice({ ...choice, cpe_id: e.target.value })
                    }
                  >
                    <option value="">Selecione</option>
                    {devices
                      .filter((d) => d.role === 'cpe')
                      .map((d) => (
                        <option key={d.id} value={d.id}>
                          {d.name} · {d.address}
                        </option>
                      ))}
                  </select>
                </label>
                <label>
                  AP{' '}
                  <select
                    required
                    aria-label="AP do vínculo"
                    value={choice.ap_id}
                    onChange={(e) =>
                      setChoice({ ...choice, ap_id: e.target.value })
                    }
                  >
                    <option value="">Selecione</option>
                    {devices
                      .filter((d) => d.role === 'ap')
                      .map((d) => (
                        <option key={d.id} value={d.id}>
                          {d.name} · {d.address}
                        </option>
                      ))}
                  </select>
                </label>
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
                      onChange={(e) =>
                        setChoice({ ...choice, [key]: e.target.value })
                      }
                    />
                  </label>
                ))}
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
          {radio.busy ? (
            <button className="primary" onClick={() => void radio.stop()}>
              Parar
            </button>
          ) : (
            link &&
            !editing && (
              <div className="radio-actions">
                <button
                  className="primary"
                  disabled={blocked}
                  onClick={() => void radio.start('once')}
                >
                  Consultar enlace
                </button>
                <button
                  disabled={blocked}
                  onClick={() => void radio.start('measure')}
                >
                  Medir custo do live
                </button>
                <button
                  disabled={blocked || !cost?.enabled}
                  onClick={() => void radio.start('live')}
                >
                  Live curto
                </button>
              </div>
            )
          )}
          {link && !editing && (
            <p>
              {cost
                ? `${cost.reason} Intervalo: ${formatMetric(cost.interval_seconds, 's')} · p95 ${formatMetric(cost.p95_seconds, 's')}`
                : 'Live desligado: custo do caminho completo ainda não medido nesta execução.'}
            </p>
          )}
          {radio.busy && (
            <p role="status">
              Consulta em curso; um enlace por vez. A janela do live termina em
              até 90 s.
            </p>
          )}
          {(radio.view?.reading || radio.view?.cpe_reading) && (
            <div className="radio-sides">
              <Reading
                title="Lado AP"
                reading={radio.view?.reading}
                code={radio.view?.ap_code}
                onlyMAC={link?.mac}
                note={false}
              />
              <Reading
                title="Lado CPE"
                reading={radio.view?.cpe_reading}
                code={radio.view?.cpe_code}
                note={false}
              />
            </div>
          )}
          {(radio.view?.reading || radio.view?.cpe_reading) && (
            <p className="radio-note">
              Taxa negociada, não é velocidade. Dados ausentes permanecem
              indisponíveis.
            </p>
          )}
        </>
      )}
    </section>
  );
}
