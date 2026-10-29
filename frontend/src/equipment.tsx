import { useDisplayNow } from './equipment-snapshot';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { PPPoE } from './pppoe';
import { EquipmentInsights } from './equipment-insights';
import { RadioEquipment } from './radio';
import {
  uptimeTitle,
  formatDuration,
  formatDate,
  exactVolume,
  formatMetric,
  formatRouterDuration,
  formatVolumePair,
} from './network-format';
import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import type { ReactNode } from 'react';
import {
  fieldForEquipmentCode,
  fieldMessages,
  validEquipmentAddress,
  validEquipmentText,
  validEquipmentSecret,
} from './equipment-validation';
import { Button, Dialog } from './desktop';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type {
  Credential,
  Device,
  DeviceInput,
  CredentialInput,
  EquipmentResult,
  Job,
} from './equipment-api';
import {
  pendingStarts,
  pendingEnrollments,
  acceptEquipmentAuthority,
} from './equipment-receipts';
const id = () => crypto.randomUUID();
const stamp = (d: Device) =>
  d.result?.observed_at ? formatDate(d.result.observed_at) : 'Não consultado';
function latestSavedReading(d: Device) {
  const monitorAt = Date.parse(d.monitoring_at || ''),
    summaryAt = Date.parse(d.result?.observed_at || '');
  if (
    d.monitoring &&
    Number.isFinite(monitorAt) &&
    (!Number.isFinite(summaryAt) || monitorAt >= summaryAt)
  )
    return {
      at: d.monitoring_at!,
      version: d.monitoring.version,
      cpu: d.monitoring.cpu,
      uptime: d.monitoring.uptime.known
        ? formatDuration(d.monitoring.uptime.min / 1e9)
        : 'Indisponível',
      exactUptime: uptimeTitle(d.monitoring.uptime),
    };
  return {
    at: d.result?.observed_at || '',
    version: d.result?.version,
    cpu: d.result?.cpu_percent,
    uptime: formatRouterDuration(d.result?.uptime),
    exactUptime: d.result?.uptime,
  };
}
type DraftState = { dirty: boolean; blocked: boolean };
const DraftProtection = createContext<{
  register: (read: () => DraftState) => void;
  cancel: () => void;
  attempt: (action: () => void) => void;
} | null>(null);
function ProtectedDialog({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: ReactNode;
}) {
  const guard = useRef<() => DraftState>(() => ({
    dirty: false,
    blocked: false,
  }));
  const [confirm, setConfirm] = useState(false);
  const pendingAction = useRef(close);
  const attempt = (action: () => void) => {
    const state = guard.current();
    if (state.blocked) return;
    pendingAction.current = action;
    if (state.dirty) setConfirm(true);
    else action();
  };
  const requestClose = () => attempt(close);
  return (
    <DraftProtection.Provider
      value={{
        register: (read) => {
          guard.current = read;
        },
        cancel: requestClose,
        attempt,
      }}
    >
      <Dialog
        portal
        title={title}
        close={requestClose}
        dismissOnBackdrop={false}
        inactive={confirm}
      >
        {children}
      </Dialog>
      {confirm && (
        <Dialog
          portal
          title="Descartar alterações?"
          close={() => setConfirm(false)}
          dismissOnBackdrop={false}
        >
          <p>As alterações deste formulário serão perdidas.</p>
          <div className="dialog-actions">
            <Button onClick={() => setConfirm(false)}>
              Continuar editando
            </Button>
            <Button
              kind="primary"
              onClick={() => {
                if (!guard.current().blocked) {
                  setConfirm(false);
                  pendingAction.current();
                }
              }}
            >
              Descartar alterações
            </Button>
          </div>
        </Dialog>
      )}
    </DraftProtection.Provider>
  );
}
function useDraftProtection(read: () => DraftState) {
  const protection = useContext(DraftProtection);
  useLayoutEffect(() => {
    protection?.register(read);
  });
  return protection?.cancel;
}
function useFieldErrors(disabled: boolean) {
  const prefix = useId();
  const ref = useRef<HTMLFormElement>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  // A daemon reply may arrive while the fieldset is still disabled by perform.
  useLayoutEffect(() => {
    const field = Object.keys(errors)[0];
    if (field) {
      const control = ref.current?.querySelector<HTMLElement>(
        `[name="${field}"]:not(:disabled)`,
      );
      control?.closest('details')?.setAttribute('open', '');
      control?.focus();
    }
  }, [errors, disabled]);
  const attrs = (name: string) => ({
    name,
    'aria-invalid': errors[name] ? true : undefined,
    'aria-describedby': errors[name] ? `${prefix}-${name}` : undefined,
  });
  const message = (name: string) =>
    errors[name] ? (
      <span id={`${prefix}-${name}`} className="field-error">
        {errors[name]}
      </span>
    ) : null;
  return { ref, errors, setErrors, attrs, message };
}
export function Equipment({
  profile,
  activation,
  connected,
  createRequest,
  primaryInHeader = false,
  onTotal,
  targetID,
  onSelect,
  onDetailChange,
}: {
  profile: string;
  activation: string;
  connected: boolean;
  createRequest: number;
  primaryInHeader?: boolean;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
}) {
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
  }, [profile, activation]);
  const [credentialRefresh, reloadCredentials] = useState(0);
  const [credentialError, setCredentialError] = useState('');
  const [ppOpen, setPPOpen] = useState(false);
  const detailSurface = useRef<HTMLElement>(null);
  const displayNow = useDisplayNow();
  const [devices, setDevices] = useState<Device[]>([]);
  const [credentialCatalog, setCredentialCatalog] = useState<{
    key: string;
    value: Credential[];
  }>({ key: '', value: [] });
  const credentials =
    credentialCatalog.key === [profile, activation].join('|')
      ? credentialCatalog.value
      : [];
  const [selected, setSelected] = useState<Device>(),
    [search, setSearch] = useState(''),
    [inactive, setInactive] = useState(false);
  const [cursor, setCursor] = useState(0),
    [next, setNext] = useState(0),
    [refresh, reload] = useState(0);
  const [total, setTotal] = useState(0);
  const [ready, setReady] = useState(false),
    [vault, setVault] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('');
  const [listError, setListError] = useState('');
  const [form, setForm] = useState<Device | 'new'>(),
    [credentialForm, setCredentialForm] = useState<Credential | 'new'>();
  const [job, setJob] = useState<Job>();
  const [detailOpen, setDetailOpen] = useState(Boolean(targetID));
  useLayoutEffect(() => {
    if (detailOpen && selected?.id && !ppOpen) {
      detailSurface.current?.focus({ preventScroll: true });
      detailSurface.current?.scrollIntoView?.({ block: 'start' });
    }
  }, [detailOpen, selected?.id, ppOpen]);
  const [connectRequest, setConnectRequest] = useState<Device>();
  const [resetTrust, setResetTrust] = useState(false);
  const resetOperation = useRef('');
  const [jobDevice, setJobDevice] = useState<Device>();
  const confirmOperation = useRef({ job: '', operation: '' });
  const detailRead = useRef(0);
  const surface = useRef(0);
  const mounted = useRef(true),
    generation = useRef(0);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [consumedCreate, consumeCreate] = useState(0);
  const shownForm =
    createRequest > 0 && createRequest !== consumedCreate ? 'new' : form;
  const closeForm = () => {
    setError('');
    setForm(undefined);
    consumeCreate(createRequest);
  };
  useLayoutEffect(() => {
    surface.current += 1;
  }, [selected?.id, shownForm, credentialForm, profile, activation]);
  useEffect(() => {
    let stopped = false;
    const read = ++generation.current;
    if (!profile || !connected) return;
    void (async () => {
      try {
        const r = await api.list(profile, activation, search, cursor, inactive);
        if (stopped || read !== generation.current) return;
        if (r.code !== 'OK' || !r.data) {
          setListError(equipmentMessage(r.code));
          return;
        }
        setListError('');
        setDevices(r.data.devices || []);
        setTotal(r.data.total_devices ?? r.data.devices.length);
        onTotal?.(
          [profile, activation].join('|'),
          r.data.total_devices ?? r.data.devices.length,
        );
        setNext(r.data.next_cursor);
        setVault(r.data.vault_available);
        setReady(true);
        setSelected((old) => {
          // List replies are summaries; only get/mutation replies own an open detail.
          if (detailOpen) return old;
          const current =
            r.data?.devices?.find((d) => d.id === old?.id) ||
            (old && detailOpen ? old : r.data?.devices?.[0]);
          return old &&
            current &&
            old.id === current.id &&
            old.revision === current.revision &&
            old.result?.observed_at === current.result?.observed_at &&
            (old.result?.interfaces?.length || detailOpen)
            ? old
            : current;
        });
      } catch {
        if (!stopped && read === generation.current)
          setListError(equipmentMessage('SERVICE_UNAVAILABLE'));
      }
    })();
    return () => {
      stopped = true;
    };
  }, [
    profile,
    activation,
    search,
    cursor,
    inactive,
    connected,
    refresh,
    detailOpen,
    onTotal,
  ]);
  useEffect(() => {
    if (!profile || !connected) return;
    let stopped = false;
    void (async () => {
      const all: Credential[] = [];
      let page = 0;
      do {
        const r = await api.credentials(profile, activation, page);
        if (stopped) return;
        if (r.code !== 'OK' || !r.data) throw new Error(r.code);
        all.push(...(r.data.credentials || []));
        page = r.data.next_cursor;
      } while (page);
      setCredentialCatalog({
        key: [profile, activation].join('|'),
        value: all,
      });
      setCredentialError('');
    })().catch(() => {
      if (!stopped) setCredentialError(equipmentMessage('SERVICE_UNAVAILABLE'));
    });
    return () => {
      stopped = true;
    };
  }, [profile, activation, connected, credentialRefresh]);
  useEffect(() => {
    if (!job || job.state !== 'RUNNING') return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    const intent = surface.current;
    const failed = (code: string) => {
      if (intent === surface.current)
        setError(
          equipmentMessage(code) +
            ' Estado da consulta ainda não confirmado; retome pelo mesmo botão, sem nova autenticação automática.',
        );
      if (++failures < 3) timer = setTimeout(() => void poll(), 250);
      else setJob(undefined);
    };
    const poll = async () => {
      try {
        const r = await api.status(profile, activation, job.id);
        if (stopped) return;
        if (r.code !== 'OK' || !r.data?.job) {
          failed(r.code);
          return;
        }
        const current = r.data.job;
        if (failures && intent === surface.current) setError('');
        failures = 0;
        if (current.state !== 'RUNNING') {
          for (const [key, operation] of pendingStarts) {
            if (
              key.startsWith(
                [
                  profile,
                  activation,
                  current.kind === 'trust.inspect' ? 'inspect' : 'query',
                  current.device_id,
                ].join('|') + '|',
              )
            ) {
              if (operation) pendingStarts.delete(key);
            }
          }
        }
        if (current.state !== 'RUNNING')
          setJob(intent === surface.current ? current : undefined);
        if (current.state === 'RUNNING')
          timer = setTimeout(() => void poll(), 250);
        else {
          if (current.code !== 'OK' && intent === surface.current)
            setError(equipmentMessage(current.code));
          if (current.kind === 'device.query-start') reload((n) => n + 1);
        }
      } catch {
        if (!stopped) failed('SERVICE_UNAVAILABLE');
      }
    };
    timer = setTimeout(() => void poll(), 150);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [job, profile, activation]);
  async function perform(
    call: () => Promise<EquipmentResult>,
    after?: (r: EquipmentResult) => void,
    refreshList = true,
    inlineFields = false,
  ) {
    if (busy) return;
    setBusy(true);
    setError('');
    const intent = surface.current;
    try {
      const r = await call();
      if (!mounted.current) return;
      if (r.code === 'OK') {
        if (refreshList) reload((n) => n + 1);
        if (intent === surface.current) after?.(r);
      } else if (
        intent === surface.current &&
        !(inlineFields && fieldForEquipmentCode[r.code])
      )
        setError(equipmentMessage(r.code));
      return r;
    } catch {
      if (mounted.current && intent === surface.current)
        setError(equipmentMessage('SERVICE_UNAVAILABLE'));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function startOperation(kind: 'query' | 'inspect', device: Device) {
    const key = [profile, activation, kind, device.id, device.revision].join(
      '|',
    );
    // Only a strictly newer verified revision retires the former payload.
    // A stale lower-revision surface cannot invalidate a newer uncertain ID.
    for (const operationKind of ['query', 'inspect']) {
      const prefix =
        [profile, activation, operationKind, device.id].join('|') + '|';
      for (const old of pendingStarts.keys()) {
        const oldRevision = Number(old.slice(prefix.length));
        if (
          old.startsWith(prefix) &&
          Number.isSafeInteger(oldRevision) &&
          Number.isSafeInteger(device.revision) &&
          oldRevision < device.revision
        )
          pendingStarts.delete(old);
      }
    }
    if (!pendingStarts.has(key) && pendingStarts.size >= 128) {
      setError(equipmentMessage('LIMIT_EXCEEDED'));
      return;
    }
    const operation = pendingStarts.get(key) || id();
    pendingStarts.set(key, operation);
    await perform(
      async () => {
        const r = await (kind === 'query' ? api.start : api.inspect)(
          profile,
          activation,
          device.id,
          operation,
          device.revision,
        );
        if (!['SERVICE_UNAVAILABLE', 'DEADLINE_EXCEEDED'].includes(r.code))
          if (r.code !== 'OK' || r.data?.job?.state !== 'RUNNING')
            pendingStarts.delete(key);
        return r;
      },
      (r) => {
        setJobDevice(device);
        setJob(r.data?.job);
        if (r.data?.job && r.data.job.code !== 'OK')
          setError(equipmentMessage(r.data.job.code));
      },
    );
  }
  useEffect(() => {
    if (!targetID || !connected) return;
    let cancelled = false;
    const read = ++detailRead.current;
    const intent = surface.current;
    void api
      .get(profile, activation, targetID)
      .then((r) => {
        if (
          cancelled ||
          read !== detailRead.current ||
          intent !== surface.current
        )
          return;
        if (r.code === 'OK' && r.data?.device?.id === targetID) {
          setSelected(r.data.device);
          setDetailOpen(true);
        } else
          setError(
            equipmentMessage(r.code === 'OK' ? 'REVISION_CONFLICT' : r.code),
          );
      })
      .catch(() => {
        if (
          !cancelled &&
          read === detailRead.current &&
          intent === surface.current
        )
          setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      });
    return () => {
      cancelled = true;
    };
  }, [targetID, profile, activation, connected]);
  useEffect(() => {
    if (!detailOpen || !selected?.id || !connected) return;
    let cancelled = false;
    const read = ++detailRead.current;
    const intent = surface.current;
    const requested = selected.id;
    void api
      .get(profile, activation, requested)
      .then((r) => {
        if (
          cancelled ||
          read !== detailRead.current ||
          intent !== surface.current
        )
          return;
        if (r.code === 'OK' && r.data?.device?.id === requested)
          setSelected(r.data.device);
        else
          setError(
            equipmentMessage(r.code === 'OK' ? 'REVISION_CONFLICT' : r.code),
          );
      })
      .catch(() => {
        if (
          !cancelled &&
          read === detailRead.current &&
          intent === surface.current
        )
          setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      });
    return () => {
      cancelled = true;
    };
  }, [
    detailOpen,
    selected?.id,
    selected?.revision,
    profile,
    activation,
    connected,
  ]);
  const queuedConnect = useEffectEvent((device: Device) => {
    setConnectRequest(undefined);
    void startOperation(device.fingerprint ? 'query' : 'inspect', device);
  });
  useEffect(() => {
    if (!connectRequest || busy || !connected) return;
    const device = connectRequest;
    let stopped = false;
    queueMicrotask(() => {
      if (!stopped) queuedConnect(device);
    });
    return () => {
      stopped = true;
    };
  }, [connectRequest, busy, connected]);
  useEffect(() => {
    if (!job || job.kind !== 'device.query-start' || job.state === 'RUNNING')
      return;
    let cancelled = false;
    const intent = surface.current;
    void api
      .get(profile, activation, job.device_id)
      .then((r) => {
        if (cancelled || intent !== surface.current) return;
        if (r.code === 'OK' && r.data?.device?.id === job.device_id) {
          setSelected(r.data.device);
          setDetailOpen(true);
          onSelect?.(r.data.device.id);
        } else setError(equipmentMessage(r.code));
      })
      .catch(() => {
        if (!cancelled && intent === surface.current)
          setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      });
    return () => {
      cancelled = true;
    };
  }, [job, profile, activation, onSelect]);
  const openDevice = (device: Device) => {
    setSelected(device);
    setDetailOpen(true);
    setJob(undefined);
    setError('');
    onSelect?.(device.id);
  };
  const dismissTrust = () => {
    if (!job || busy) return;
    void perform(
      () => api.cancel(profile, activation, job.id),
      () => setJob(undefined),
      false,
    );
  };
  const showSavedDevice = (r: EquipmentResult) => {
    closeForm();
    if (r.data?.device) {
      setSelected(r.data.device);
      setDetailOpen(true);
      onSelect?.(r.data.device.id);
    }
  };
  const continueEnrollment = (r: EquipmentResult) => {
    showSavedDevice(r);
    if (r.data?.device) setConnectRequest(r.data.device);
  };
  useEffect(() => {
    onDetailChange?.(detailOpen);
  }, [detailOpen, onDetailChange]);
  if (!profile)
    return (
      <div className="empty-state">
        <h2>Selecione uma empresa</h2>
        <p>O inventário pertence à empresa ativa.</p>
      </div>
    );
  const locked = busy || !connected;
  const trustOpen = Boolean(
    !selected?.fingerprint &&
    job?.kind === 'trust.inspect' &&
    job.state === 'SUCCEEDED' &&
    selected &&
    job.device_id === selected.id &&
    jobDevice?.revision === selected.revision,
  );
  const accessDetails = selected && (
    <>
      <div className="kv">
        <span className="key">Endereço</span>
        <span className="mono">
          {selected.address}:{selected.port}
        </span>
      </div>
      <div className="kv">
        <span className="key">Credencial de consulta</span>
        <span>
          {credentials.find((c) => c.id === selected.credential_id)?.alias ||
            (selected.credential_id ? 'Acesso salvo' : 'Não vinculada')}
        </span>
      </div>
      <div className="kv">
        <span className="key">Última leitura válida</span>
        <span>{stamp(selected)}</span>
      </div>
      <div className="kv">
        <span className="key">Identidade do destino</span>
        <span>
          {selected.fingerprint ? 'Chave salva' : 'Primeira conexão pendente'}
        </span>
      </div>
      {selected.fingerprint && (
        <details className="equipment-help">
          <summary>Detalhes da chave SSH</summary>
          <p className="fingerprint mono">{selected.fingerprint}</p>
          <Button
            disabled={locked || job?.state === 'RUNNING'}
            onClick={() => {
              resetOperation.current = id();
              setResetTrust(true);
            }}
          >
            Redefinir confiança
          </Button>
        </details>
      )}
      {detailOpen && (
        <Button
          disabled={locked || !selected.active || job?.state === 'RUNNING'}
          onClick={() => void startOperation('inspect', selected)}
        >
          Verificar identidade
        </Button>
      )}
    </>
  );
  const vaultNotice =
    'Cofre indisponível. Confira a credencial do serviço nesta instalação.';
  if (ppOpen && selected)
    return (
      <PPPoE
        key={[profile, activation, selected.id].join('|')}
        profile={profile}
        activation={activation}
        device={selected}
        back={() => {
          setPPOpen(false);
        }}
      />
    );
  return (
    <div className="equipment-capability">
      {credentialError && (
        <p role="alert">
          {credentialError}{' '}
          <button onClick={() => reloadCredentials((n) => n + 1)}>
            Recarregar credenciais
          </button>
        </p>
      )}
      {error && !shownForm && !credentialForm && !trustOpen && !resetTrust && (
        <p className="inline-error operation-error" role="alert">
          {error}
        </p>
      )}
      {listError &&
        !shownForm &&
        !credentialForm &&
        !trustOpen &&
        !resetTrust && (
          <div className="inventory-notice">
            <p className="inline-error" role="alert">
              {listError}
            </p>
            <Button disabled={locked} onClick={() => reload((n) => n + 1)}>
              Tentar novamente
            </Button>
          </div>
        )}
      {!vault &&
        ready &&
        !error &&
        !listError &&
        !shownForm &&
        !credentialForm &&
        !trustOpen && <p className="inventory-notice">{vaultNotice}</p>}
      <div
        className={
          'pane-layout ' +
          (detailOpen
            ? 'equipment-detail-layout'
            : !selected
              ? 'inventory-without-detail'
              : '')
        }
      >
        <section className="pane-main" hidden={detailOpen}>
          {total === 0 && !search && !inactive && (
            <Button
              onClick={() => {
                setError('');
                setCredentialForm('new');
              }}
              disabled={!connected || !vault}
            >
              Credenciais
            </Button>
          )}
          {(total > 0 || search || inactive) && (
            <div className="inventory-tools">
              {!primaryInHeader && (
                <Button
                  kind="primary"
                  onClick={() => setForm('new')}
                  disabled={locked}
                >
                  Adicionar equipamento
                </Button>
              )}
              <input
                aria-label="Buscar equipamento"
                placeholder="Buscar equipamento"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setCursor(0);
                }}
              />
              <select
                aria-label="Estado dos equipamentos"
                value={inactive ? 'inactive' : 'active'}
                onChange={(e) => {
                  setInactive(e.target.value === 'inactive');
                  setCursor(0);
                }}
              >
                <option value="active">Ativos</option>
                <option value="inactive">Inativos</option>
              </select>
              <Button
                onClick={() => {
                  setError('');
                  setCredentialForm('new');
                }}
                disabled={!connected || !vault}
              >
                Credenciais
              </Button>
            </div>
          )}
          {devices.length > 0 && (
            <div
              className="table-scroll"
              tabIndex={0}
              aria-label="Lista de equipamentos"
            >
              <table className="app-table inventory-table">
                <thead>
                  <tr>
                    <th>Equipamento</th>
                    <th>Endereço</th>
                    <th className="os-cell">Sistema</th>
                    <th>Papel</th>
                    <th>Uptime</th>
                    <th>CPU</th>
                    <th>Consulta</th>
                    <th>Última leitura</th>
                  </tr>
                </thead>
                <tbody>
                  {devices.map((d) => {
                    const saved = latestSavedReading(d);
                    return (
                      <tr
                        key={d.id}
                        className={
                          'selectable ' +
                          (selected?.id === d.id ? 'selected' : '')
                        }
                        tabIndex={0}
                        aria-selected={selected?.id === d.id}
                        onClick={() => {
                          openDevice(d);
                        }}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault();
                            openDevice(d);
                          }
                        }}
                      >
                        <td>
                          <span className="cell-name">{d.name}</span>
                        </td>
                        <td className="mono">{d.address}</td>
                        <td className="os-cell">
                          {saved.version || 'Desconhecido'}
                        </td>
                        <td>
                          {d.role === 'concentrator'
                            ? 'Concentrador PPPoE'
                            : d.role || 'Não definido'}
                        </td>
                        <td title={saved.exactUptime}>{saved.uptime}</td>
                        <td>{formatMetric(saved.cpu, '%')}</td>
                        <td>
                          <span
                            className={
                              'text-state ' +
                              (d.result || d.monitoring ? 'good' : 'muted')
                            }
                          >
                            {d.active
                              ? d.result || d.monitoring
                                ? 'Leitura disponível'
                                : 'Não consultado'
                              : 'Inativo'}
                          </span>
                        </td>
                        <td className="reading-time" title={saved.at}>
                          {saved.at ? formatDate(saved.at) : 'Não consultado'}
                          {saved.at && (
                            <small className="cell-sub">
                              Últimos dados conhecidos · há{' '}
                              {formatDuration(
                                (displayNow - Date.parse(saved.at)) / 1000,
                              )}
                            </small>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
          {!devices.length && ready && !listError && (
            <div className="empty-state">
              <h2>
                {search
                  ? 'Nenhum resultado'
                  : inactive
                    ? 'Nenhum equipamento inativo'
                    : total
                      ? 'Nenhum equipamento ativo'
                      : 'Nenhum equipamento cadastrado'}
              </h2>
              {search ? (
                <Button
                  onClick={() => {
                    setSearch('');
                    setCursor(0);
                  }}
                >
                  Limpar busca
                </Button>
              ) : total > 0 && !inactive ? (
                <Button
                  onClick={() => {
                    setInactive(true);
                    setCursor(0);
                  }}
                >
                  Mostrar inativos
                </Button>
              ) : !total && !inactive ? (
                <Button
                  kind="primary"
                  onClick={() => setForm('new')}
                  disabled={locked}
                >
                  Adicionar equipamento
                </Button>
              ) : null}
            </div>
          )}
          {(cursor > 0 || next > 0) && (
            <div className="list-footer">
              <span>{devices.length} equipamentos nesta página</span>
              <div>
                <Button disabled={cursor === 0} onClick={() => setCursor(0)}>
                  Início
                </Button>
                <Button disabled={!next} onClick={() => setCursor(next)}>
                  Próxima
                </Button>
              </div>
            </div>
          )}
        </section>
        {selected && (
          <aside
            ref={detailSurface}
            tabIndex={-1}
            className={detailOpen ? 'equipment-detail' : 'inspector'}
          >
            {selected && (
              <>
                {detailOpen && (
                  <Button
                    kind="flat"
                    onClick={() => {
                      surface.current++;
                      detailRead.current++;
                      setDetailOpen(false);
                      setJob(undefined);
                    }}
                  >
                    Voltar aos equipamentos
                  </Button>
                )}
                <p className="inspector-heading">Detalhes do equipamento</p>
                <h2>{selected.name}</h2>
                <p>
                  Papel declarado:{' '}
                  {
                    (
                      {
                        concentrator: 'Concentrador',
                        ap: 'AP',
                        cpe: 'CPE',
                        ptp: 'PTP',
                        other: 'Outro',
                      } as Record<string, string>
                    )[selected.role || 'other']
                  }
                </p>
                {(!selected.role || selected.role === 'other') && (
                  <p>
                    Para ver os logins de clientes PPPoE, edite o cadastro e
                    declare o papel Concentrador.
                  </p>
                )}
                <p className="subtitle">
                  SSH · {selected.result?.family || 'Sistema desconhecido'}
                </p>
                {!detailOpen && accessDetails}
                <div className="bottom-actions equipment-actions">
                  {selected.role === 'concentrator' &&
                    (!detailOpen || !selected.result) && (
                      <Button
                        kind={detailOpen ? 'primary' : undefined}
                        disabled={!connected || !selected.fingerprint}
                        onClick={() => {
                          setDetailOpen(true);
                          setPPOpen(true);
                        }}
                      >
                        Abrir sessões PPPoE
                      </Button>
                    )}
                  {selected.role === 'ptp' ? null : (selected.role === 'ap' ||
                      selected.role === 'cpe') &&
                    selected.fingerprint ? (
                    !detailOpen && (
                      <Button
                        kind="primary"
                        onClick={() => setDetailOpen(true)}
                      >
                        Abrir rádio
                      </Button>
                    )
                  ) : (
                    <Button
                      kind={
                        detailOpen && selected.role !== 'concentrator'
                          ? 'primary'
                          : undefined
                      }
                      disabled={
                        locked ||
                        !vault ||
                        !selected.active ||
                        !selected.credential_id ||
                        job?.state === 'RUNNING'
                      }
                      onClick={() => {
                        setDetailOpen(true);
                        void startOperation(
                          selected.fingerprint ? 'query' : 'inspect',
                          selected,
                        );
                      }}
                    >
                      {detailOpen
                        ? selected.result
                          ? 'Atualizar resumo'
                          : 'Consultar resumo'
                        : selected.result
                          ? 'Atualizar'
                          : 'Conectar'}
                    </Button>
                  )}
                  {!detailOpen && selected.role !== 'ptp' && (
                    <Button
                      disabled={
                        locked || !selected.active || job?.state === 'RUNNING'
                      }
                      onClick={() => void startOperation('inspect', selected)}
                    >
                      Verificar identidade
                    </Button>
                  )}
                  {!detailOpen && (
                    <Button
                      kind="flat"
                      disabled={locked}
                      onClick={() => openDevice(selected)}
                    >
                      Abrir equipamento
                    </Button>
                  )}
                  <Button
                    kind="flat"
                    disabled={!connected}
                    onClick={() => {
                      setError('');
                      setForm(selected);
                    }}
                  >
                    Editar equipamento
                  </Button>
                  <Button
                    kind="flat"
                    disabled={locked}
                    onClick={() =>
                      void perform(
                        () =>
                          api.active(
                            profile,
                            activation,
                            selected.id,
                            id(),
                            selected.revision,
                            !selected.active,
                          ),
                        (r) => {
                          if (r.data?.device) setSelected(r.data.device);
                        },
                      )
                    }
                  >
                    {selected.active ? 'Desativar' : 'Reativar'}
                  </Button>
                </div>
                {selected.last_attempt && (
                  <p className="inventory-notice" role="status">
                    Última tentativa: {formatDate(selected.last_attempt.at)} ·{' '}
                    {selected.last_attempt.state === 'SUCCEEDED'
                      ? 'Leitura concluída'
                      : selected.last_attempt.state === 'STARTED'
                        ? 'Iniciada; conclusão ainda não registrada'
                        : equipmentMessage(selected.last_attempt.code)}
                  </p>
                )}
                {selected.result && (
                  <EquipmentReading
                    key={selected.id + selected.result.observed_at}
                    device={selected}
                    focus={false}
                    separate={detailOpen}
                    openPPPoE={
                      selected.role === 'concentrator'
                        ? () => setPPOpen(true)
                        : undefined
                    }
                  />
                )}
                {detailOpen && selected.role === 'concentrator' && (
                  <EquipmentInsights
                    profile={profile}
                    activation={activation}
                    device={selected}
                    blocked={
                      locked ||
                      !vault ||
                      !selected.active ||
                      !selected.fingerprint
                    }
                  />
                )}
                {detailOpen && selected.role === 'concentrator' && (
                  <PPPoEReadPanel
                    profile={profile}
                    activation={activation}
                    device={selected}
                    observation=""
                    rows={[]}
                    health
                    blocked={
                      locked ||
                      !vault ||
                      !selected.active ||
                      !selected.fingerprint
                    }
                  />
                )}
                {detailOpen &&
                  (selected.role === 'ap' || selected.role === 'cpe') &&
                  selected.fingerprint && (
                    <RadioEquipment
                      key={[
                        profile,
                        activation,
                        selected.id,
                        selected.revision,
                      ].join('|')}
                      profile={profile}
                      activation={activation}
                      device={selected}
                      blocked={locked || !vault || !selected.active}
                    />
                  )}
                {detailOpen && (
                  <details className="equipment-access">
                    <summary>Acesso e identidade</summary>
                    {accessDetails}
                  </details>
                )}
              </>
            )}
          </aside>
        )}
      </div>
      {detailOpen && !selected && <p role="status">Carregando equipamento…</p>}
      {job?.state === 'RUNNING' && (
        <div className="inventory-notice" role="status">
          {job.kind === 'trust.inspect'
            ? 'Conferindo chave SSH sem enviar senha…'
            : 'Consulta SSH em curso…'}{' '}
          <Button
            onClick={() =>
              void perform(
                () => api.cancel(profile, activation, job.id),
                (r) => setJob(r.data?.job),
              )
            }
          >
            Cancelar consulta
          </Button>
        </div>
      )}
      {trustOpen && selected && job && (
        <Dialog portal title="Primeira conexão" close={dismissTrust}>
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          <p>
            Confiar em {selected.name} ({selected.address}:{selected.port}) e
            conectar?
          </p>
          <p>
            Esta é a primeira conexão. Mudanças posteriores na chave bloquearão
            a conexão.
          </p>
          <details className="equipment-help">
            <summary>Detalhes avançados</summary>
            <p>A primeira chave ainda não foi verificada por outro canal.</p>
            <p className="fingerprint mono">{job.fingerprint}</p>
          </details>
          <div className="dialog-actions">
            <Button disabled={locked} onClick={dismissTrust}>
              Cancelar
            </Button>
            <Button
              kind="primary"
              disabled={locked}
              onClick={() => {
                if (confirmOperation.current.job !== job.id)
                  confirmOperation.current = { job: job.id, operation: id() };
                void perform(
                  () =>
                    api.confirm(
                      profile,
                      activation,
                      selected.id,
                      confirmOperation.current.operation,
                      job.id,
                      job.fingerprint || '',
                      selected.revision,
                    ),
                  (r) => {
                    setJob(undefined);
                    if (r.data?.device) {
                      setSelected(r.data.device);
                      setConnectRequest(r.data.device);
                    }
                  },
                  false,
                );
              }}
            >
              Confiar neste equipamento e conectar
            </Button>
          </div>
        </Dialog>
      )}
      {resetTrust && selected && (
        <Dialog
          portal
          title="Redefinir confiança?"
          close={() => {
            if (!busy) setResetTrust(false);
          }}
        >
          <p>
            Use somente após confirmar uma troca legítima de chave no destino{' '}
            {selected.address}:{selected.port}. A próxima conexão exigirá um
            novo aceite; isto não verifica a identidade por outro canal.
          </p>
          {error && (
            <p role="alert" className="inline-error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button disabled={locked} onClick={() => setResetTrust(false)}>
              Cancelar
            </Button>
            <Button
              kind="primary"
              disabled={locked}
              onClick={() =>
                void perform(
                  () =>
                    api.resetTrust(
                      profile,
                      activation,
                      selected.id,
                      resetOperation.current,
                      selected.revision,
                    ),
                  (r) => {
                    if (r.data?.device) setSelected(r.data.device);
                    setResetTrust(false);
                    setJob(undefined);
                  },
                  false,
                )
              }
            >
              Remover chave salva
            </Button>
          </div>
        </Dialog>
      )}
      {shownForm && (
        <ProtectedDialog
          title={
            shownForm === 'new' ? 'Adicionar equipamento' : 'Editar equipamento'
          }
          close={closeForm}
        >
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          {!vault && ready && !error && (
            <p role="alert" className="inline-error">
              {vaultNotice}
            </p>
          )}
          <DeviceForm
            key={shownForm === 'new' ? 'new' : shownForm.id}
            device={shownForm === 'new' ? undefined : shownForm}
            credentials={credentials}
            disabled={locked}
            vault={vault}
            contextKey={[profile, activation].join('|')}
            save={(d) =>
              perform(
                async () => {
                  const r = await api.save(profile, activation, d);
                  if (r.code === 'OK') reloadCredentials((n) => n + 1);
                  return r;
                },
                shownForm === 'new' ? continueEnrollment : showSavedDevice,
                true,
                true,
              )
            }
            reconcile={(device, operation) =>
              perform(
                () =>
                  api.enrollmentStatus(profile, activation, device, operation),
                (r) => {
                  if (r.data?.operation_state === 'COMMITTED') {
                    reloadCredentials((n) => n + 1);
                    continueEnrollment(r);
                  }
                },
              )
            }
          />
        </ProtectedDialog>
      )}
      {credentialForm && (
        <ProtectedDialog
          title="Credenciais de consulta"
          close={() => setCredentialForm(undefined)}
        >
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          {!vault && ready && !error && (
            <p role="alert" className="inline-error">
              {vaultNotice}
            </p>
          )}
          <p>
            Reutilizáveis somente nesta empresa. Substituir aqui não altera a
            senha no equipamento.
          </p>
          <CredentialPicker
            disabled={locked}
            credentials={credentials}
            selected={credentialForm}
            select={setCredentialForm}
          />
          <CredentialForm
            key={credentialForm === 'new' ? 'new' : credentialForm.id}
            credential={credentialForm === 'new' ? undefined : credentialForm}
            disabled={locked || !vault}
            save={(d) =>
              perform(
                () => api.putCredential(profile, activation, d),
                () => {
                  setCredentialForm(undefined);
                  reloadCredentials((n) => n + 1);
                },
                true,
                true,
              )
            }
          />
        </ProtectedDialog>
      )}
    </div>
  );
}
function DeviceForm({
  device,
  credentials,
  disabled,
  vault,
  contextKey,
  save,
  reconcile,
}: {
  device?: Device;
  credentials: Credential[];
  disabled: boolean;
  vault: boolean;
  contextKey: string;
  save: (d: DeviceInput) => Promise<EquipmentResult | undefined>;
  reconcile: (
    device: string,
    operation: string,
  ) => Promise<EquipmentResult | undefined>;
}) {
  const receipt = !device ? pendingEnrollments.get(contextKey) : undefined;
  const [draft, setDraft] = useState<DeviceInput>(() => ({
    id: device?.id || receipt?.id || id(),
    operation_id: receipt?.operation_id || id(),
    revision: device?.revision || 0,
    name: device?.name || '',
    role: device?.role || 'other',
    address: device?.address || '',
    port: device?.port || 22,
    transport: 'ssh',
    credential_id: device?.credential_id || '',
    user: '',
    secret: '',
  }));
  const [existing, setExisting] = useState(Boolean(device));
  const [uncertain, setUncertain] = useState(Boolean(!device && receipt));
  const [checking, setChecking] = useState(false);
  const [retry, setRetry] = useState(false);
  const submitting = useRef(false);
  const { ref: formRef, setErrors, attrs, message } = useFieldErrors(disabled);
  const [sent, setSent] = useState(false);
  const cancel = useDraftProtection(() => ({
    dirty:
      uncertain ||
      draft.name !== (device?.name || '') ||
      draft.role !== (device?.role || 'other') ||
      draft.address !== (device?.address || '') ||
      draft.port !== (device?.port || 22) ||
      draft.credential_id !== (device?.credential_id || '') ||
      draft.user !== '' ||
      draft.secret !== '' ||
      existing !== Boolean(device),
    blocked: submitting.current || checking,
  }));
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const change = (values: Partial<DeviceInput>) =>
    setDraft({ ...draft, ...values });
  return (
    <form
      noValidate
      ref={formRef}
      onSubmit={(e) => {
        e.preventDefault();
        if (submitting.current || disabled || uncertain) return;
        const errors: Record<string, string> = {};
        if (!validEquipmentText(draft.name)) errors.name = fieldMessages.name;
        if (!validEquipmentAddress(draft.address))
          errors.address = fieldMessages.address;
        if (
          !Number.isInteger(draft.port) ||
          draft.port < 1 ||
          draft.port > 65535
        )
          errors.port = fieldMessages.port;
        if (existing && !device && !draft.credential_id)
          errors.credential_id = fieldMessages.credential_id;
        if (!existing) {
          if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
          if (!validEquipmentSecret(draft.secret))
            errors.secret = fieldMessages.secret;
        }
        setErrors(errors);
        if (Object.keys(errors).length) return;
        submitting.current = true;
        const request = {
          ...draft,
          name: draft.name.trim(),
          address: draft.address.trim(),
          user: existing ? '' : draft.user.trim(),
          secret: existing ? '' : draft.secret,
          credential_id: existing ? draft.credential_id : '',
        };
        const direct = !device;
        if (direct)
          pendingEnrollments.set(contextKey, {
            id: draft.id,
            operation_id: draft.operation_id,
          });
        setDraft({ ...draft, secret: '' });
        setSent(!existing);
        void (async () => {
          try {
            const result = await save(request);
            // No secret retained in the form, receipt registry, or retry closure.
            request.secret = '';
            if (alive.current && result && fieldForEquipmentCode[result.code]) {
              const field = fieldForEquipmentCode[result.code];
              setErrors({ [field]: fieldMessages[field] });
            }
            if (direct) {
              const unknown =
                !result ||
                [
                  'SERVICE_UNAVAILABLE',
                  'DEADLINE_EXCEEDED',
                  'REVISION_CONFLICT',
                ].includes(result.code);
              if (!unknown) pendingEnrollments.delete(contextKey);
              if (alive.current) setUncertain(unknown);
            }
          } catch {
            if (alive.current && direct) setUncertain(true);
          } finally {
            request.secret = '';
            submitting.current = false;
          }
        })();
      }}
    >
      {uncertain ? (
        <div role="status" className="inventory-notice">
          <p>
            Cadastro ainda não confirmado. Confira o resultado antes de fechar.
          </p>
          <Button
            disabled={disabled || checking}
            onClick={() => {
              setChecking(true);
              void reconcile(draft.id, draft.operation_id).then((r) => {
                if (r?.code === 'OK' && r.data?.operation_state === 'COMMITTED')
                  pendingEnrollments.delete(contextKey);
                if (!alive.current) return;
                setChecking(false);
                if (
                  r?.code === 'OK' &&
                  r.data?.operation_state === 'NOT_FOUND'
                ) {
                  // Absence is not cancellation: any deliberate retry keeps these exact IDs.
                  setUncertain(false);
                  setRetry(true);
                }
              });
            }}
          >
            Conferir cadastro
          </Button>
        </div>
      ) : (
        <>
          {retry && (
            <p role="status">
              Ainda não há confirmação. Para tentar com o mesmo cadastro,
              preencha os dados novamente.
            </p>
          )}
          <fieldset className="device-fields" disabled={disabled}>
            <label className="field">
              Nome
              <input
                aria-label="Nome"
                {...attrs('name')}
                value={draft.name}
                maxLength={80}
                required
                onChange={(e) => change({ name: e.target.value })}
              />
              {message('name')}
            </label>
            <label className="field">
              Papel do equipamento
              <select
                aria-label="Papel do equipamento"
                value={draft.role}
                onChange={(e) => change({ role: e.target.value })}
              >
                <option value="other">Outro</option>
                <option value="concentrator">Concentrador</option>
                <option value="ap">AP</option>
                <option value="cpe">CPE</option>
                <option value="ptp">PTP (somente cadastro)</option>
              </select>
            </label>
            <label className="field">
              IP de gerenciamento
              <input
                aria-label="IP de gerenciamento"
                {...attrs('address')}
                value={draft.address}
                required
                onChange={(e) => change({ address: e.target.value })}
                placeholder="IPv4 ou IPv6 literal"
              />
              {message('address')}
            </label>
            <details className="equipment-port">
              <summary>Porta SSH · {draft.port}</summary>
              <label className="field">
                Porta SSH
                <input
                  type="number"
                  aria-label="Porta SSH"
                  min={1}
                  max={65535}
                  {...attrs('port')}
                  value={draft.port}
                  required
                  onChange={(e) => change({ port: Number(e.target.value) })}
                />
                {message('port')}
              </label>
            </details>
            {existing ? (
              <>
                <label className="field">
                  Credencial de consulta
                  <select
                    aria-label="Credencial de consulta"
                    {...attrs('credential_id')}
                    value={draft.credential_id}
                    required={!device}
                    onChange={(e) => change({ credential_id: e.target.value })}
                  >
                    <option value="">
                      {device ? 'Não vinculada' : 'Selecione'}
                    </option>
                    {credentials.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.alias} · {c.user}
                      </option>
                    ))}
                  </select>
                  {message('credential_id')}
                </label>
                {!device && (
                  <p className="muted">Somente credenciais desta empresa.</p>
                )}
              </>
            ) : (
              <>
                <label className="field">
                  Usuário
                  <input
                    required
                    maxLength={80}
                    aria-label="Usuário"
                    {...attrs('user')}
                    value={draft.user}
                    autoComplete="off"
                    onChange={(e) => change({ user: e.target.value })}
                  />
                  {message('user')}
                </label>
                <label className="field">
                  Senha
                  <input
                    type="password"
                    required
                    maxLength={1024}
                    aria-label="Senha"
                    {...attrs('secret')}
                    value={draft.secret}
                    autoComplete="new-password"
                    disabled={!vault}
                    onChange={(e) => change({ secret: e.target.value })}
                  />
                  {message('secret')}
                </label>
              </>
            )}
            {!device && (
              <Button
                kind="flat"
                disabled={!vault && existing}
                onClick={() => {
                  setExisting(!existing);
                  change({ user: '', secret: '', credential_id: '' });
                }}
              >
                {existing
                  ? 'Informar usuário e senha'
                  : 'Usar credencial existente'}
              </Button>
            )}
          </fieldset>
          {device && (
            <details className="equipment-help">
              <summary>Sobre o acesso</summary>
              <p>
                A senha salva permanece intacta. Para substituir um acesso
                compartilhado, use Credenciais. Alterar IP ou porta exige
                conferir a identidade novamente.
              </p>
            </details>
          )}
          {sent && !existing && !disabled && (
            <p role="status">
              Digite a senha novamente para tentar salvar. A falha ao salvar não
              verifica a senha no equipamento.
            </p>
          )}
          <div className="dialog-actions">
            <Button disabled={disabled} onClick={cancel}>
              Cancelar
            </Button>
            <button
              className="btn primary"
              disabled={disabled || (!device && !existing && !vault)}
            >
              {device ? 'Salvar equipamento' : 'Salvar e conectar'}
            </button>
          </div>
        </>
      )}
    </form>
  );
}
function CredentialPicker({
  disabled,
  credentials,
  selected,
  select,
}: {
  disabled: boolean;
  credentials: Credential[];
  selected: Credential | 'new';
  select: (value: Credential | 'new') => void;
}) {
  const protection = useContext(DraftProtection);
  return (
    <select
      disabled={disabled}
      aria-label="Credencial para editar"
      value={selected === 'new' ? '' : selected.id}
      onChange={(e) => {
        const target =
          credentials.find((c) => c.id === e.target.value) || 'new';
        protection?.attempt(() => select(target));
      }}
    >
      <option value="">Nova credencial</option>
      {credentials.map((c) => (
        <option key={c.id} value={c.id}>
          {c.alias} · {c.links} vínculos
        </option>
      ))}
    </select>
  );
}
function CredentialForm({
  credential,
  disabled,
  save,
}: {
  credential?: Credential;
  disabled: boolean;
  save: (d: CredentialInput) => Promise<EquipmentResult | undefined>;
}) {
  const [draft, setDraft] = useState<CredentialInput>(() => ({
    id: credential?.id || id(),
    operation_id: id(),
    revision: credential?.revision || 0,
    alias: credential?.alias || '',
    user: credential?.user || '',
    secret: '',
    affected_links: credential?.links || 0,
  }));
  const [confirmed, setConfirmed] = useState(false);
  const submitting = useRef(false);
  const [sent, setSent] = useState(false);
  const { ref: formRef, setErrors, attrs, message } = useFieldErrors(disabled);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const cancel = useDraftProtection(() => ({
    dirty:
      draft.alias !== (credential?.alias || '') ||
      draft.user !== (credential?.user || '') ||
      draft.secret !== '' ||
      confirmed,
    blocked: submitting.current,
  }));
  return (
    <form
      noValidate
      ref={formRef}
      onSubmit={(e) => {
        e.preventDefault();
        if (submitting.current || disabled || (credential && !confirmed))
          return;
        const errors: Record<string, string> = {};
        if (!validEquipmentText(draft.alias))
          errors.alias = fieldMessages.alias;
        if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
        if (!validEquipmentSecret(draft.secret))
          errors.secret = fieldMessages.secret;
        setErrors(errors);
        if (Object.keys(errors).length) return;
        submitting.current = true;
        const request = {
          ...draft,
          alias: draft.alias.trim(),
          user: draft.user.trim(),
        };
        setDraft({ ...draft, secret: '' });
        setSent(true);
        void (async () => {
          try {
            const result = await save(request);
            if (alive.current && result && fieldForEquipmentCode[result.code]) {
              const field = fieldForEquipmentCode[result.code];
              setErrors({ [field]: fieldMessages[field] });
            }
          } catch {
            // perform reports transport failures; never retain the submitted secret.
          } finally {
            request.secret = '';
            submitting.current = false;
          }
        })();
      }}
    >
      <label className="field">
        Alias
        <input
          required
          maxLength={80}
          disabled={disabled}
          aria-label="Alias"
          {...attrs('alias')}
          value={draft.alias}
          onChange={(e) => setDraft({ ...draft, alias: e.target.value })}
        />
        {message('alias')}
      </label>
      <label className="field">
        Usuário remoto
        <input
          required
          maxLength={80}
          disabled={disabled}
          aria-label="Usuário remoto"
          {...attrs('user')}
          value={draft.user}
          autoComplete="off"
          onChange={(e) => setDraft({ ...draft, user: e.target.value })}
        />
        {message('user')}
      </label>
      <label className="field">
        Senha remota
        <input
          type="password"
          required
          maxLength={1024}
          autoComplete="new-password"
          disabled={disabled}
          aria-label="Senha remota"
          {...attrs('secret')}
          value={draft.secret}
          onChange={(e) => setDraft({ ...draft, secret: e.target.value })}
        />
        {message('secret')}
      </label>
      {credential && (
        <label className="check">
          <input
            disabled={disabled}
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
          />
          Substituir o acesso usado por {credential.links} equipamentos,
          inclusive inativos.
        </label>
      )}
      {sent && !disabled && (
        <p role="status">
          Digite a senha novamente para tentar salvar. A falha ao salvar não
          verifica a senha no equipamento.
        </p>
      )}
      <div className="dialog-actions">
        <Button disabled={disabled} onClick={cancel}>
          Cancelar
        </Button>
        <button
          className="btn primary"
          disabled={disabled || Boolean(credential && !confirmed)}
        >
          Salvar credencial
        </button>
      </div>
    </form>
  );
}

function EquipmentReading({
  device,
  focus,
  separate = false,
  openPPPoE,
}: {
  device: Device;
  focus: boolean;
  separate?: boolean;
  openPPPoE?: () => void;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const table = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (focus) heading.current?.focus({ preventScroll: true });
  }, [focus]);
  const showPage = (next: number) => {
    setPage(next);
    if (table.current) {
      table.current.scrollTop = 0;
      table.current.scrollIntoView({ block: 'start' });
      table.current.focus({ preventScroll: true });
    }
  };
  const [page, setPage] = useState(0);
  const [tab, setTab] = useState('summary');
  const tabs = [
    ['summary', 'Resumo'],
    ['physical', 'Interfaces físicas'],
    ['vlan', 'VLANs'],
    ['vpn', 'VPNs e túneis'],
    ['other', 'Outras interfaces'],
  ];
  const category = (kind: string) => {
    const value = kind.toLowerCase();
    if (['ether', 'wlan', 'wifi', 'w60g', 'lte'].includes(value))
      return 'physical';
    if (value === 'vlan') return 'vlan';
    if (
      /^(wireguard|pptp|l2tp|ovpn|sstp|eoip|gre|ipip|6to4|6in4|vti|vxlan)/.test(
        value,
      )
    )
      return 'vpn';
    return 'other';
  };
  const pickTab = (next: string) => {
    setTab(next);
    setPage(0);
  };
  const r = device.result!;
  const part = r.interface_part;
  const allRows = r.interfaces || [];
  const rows = separate
    ? allRows.filter((row) => category(row.kind) === tab)
    : allRows;
  return (
    <section className="equipment-result" aria-label="Resultado da consulta">
      {separate && (
        <nav
          className="equipment-reading-tabs"
          aria-label="Áreas do equipamento"
        >
          <div
            role="tablist"
            aria-label="Dados do equipamento"
            onKeyDown={(event) => {
              if (
                !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)
              )
                return;
              const controls = Array.from(
                event.currentTarget.querySelectorAll<HTMLButtonElement>(
                  '[role=tab]',
                ),
              );
              const index = controls.indexOf(event.target as HTMLButtonElement);
              if (index < 0) return;
              const next =
                event.key === 'Home'
                  ? 0
                  : event.key === 'End'
                    ? controls.length - 1
                    : (index +
                        (event.key === 'ArrowRight' ? 1 : -1) +
                        controls.length) %
                      controls.length;
              event.preventDefault();
              controls[next]?.click();
              controls[next]?.focus();
            }}
          >
            {tabs.map(([value, label]) => (
              <button
                type="button"
                key={value}
                role="tab"
                aria-selected={tab === value}
                onClick={() => pickTab(value)}
              >
                {label}
              </button>
            ))}
          </div>
          {openPPPoE && (
            <Button kind="primary" onClick={openPPPoE}>
              Abrir sessões PPPoE
            </Button>
          )}
        </nav>
      )}
      {(!separate || tab === 'summary') && (
        <>
          <h2 ref={heading} tabIndex={-1}>
            Resumo
          </h2>
          <h3>{r.identity || 'Identidade não informada'}</h3>
          <div className="equipment-summary">
            {[
              ['RouterOS', r.version || 'Não informado'],
              ['Placa / modelo', r.board || 'Não informado'],
              ['Tempo ligado', formatRouterDuration(r.uptime)],
              [
                'CPU',
                r.cpu_percent === undefined
                  ? 'Não informada'
                  : formatMetric(r.cpu_percent, '%'),
              ],
              [
                'Memória livre / total',
                formatVolumePair(r.free_memory_bytes, r.total_memory_bytes),
              ],
            ].map(([label, value]) => (
              <div className="kv" key={label}>
                <span className="key">{label}</span>
                <span
                  title={
                    label === 'Memória livre / total'
                      ? `${exactVolume(r.free_memory_bytes)} / ${exactVolume(r.total_memory_bytes)}`
                      : label === 'Tempo ligado'
                        ? r.uptime
                        : undefined
                  }
                >
                  {value}
                </span>
              </div>
            ))}
          </div>
          <p className="muted">
            Última leitura válida: {stamp(device)} ·{' '}
            {r.quality === 'complete'
              ? 'Resumo completo'
              : 'Leitura com parte limitada ou indisponível'}
          </p>
          <details className="equipment-help">
            <summary>Origem da leitura</summary>
            <p className="fingerprint">{r.source}</p>
          </details>
        </>
      )}
      {(!separate || tab !== 'summary') && (
        <>
          <h2>
            {separate
              ? tabs.find(([value]) => value === tab)?.[1]
              : 'Interfaces'}
          </h2>
          {separate && (
            <p>
              {rows.length} nesta categoria da leitura armazenada. Logins de
              clientes ficam em Sessões PPPoE.
            </p>
          )}

          {part ? (
            <>
              <p>
                {part.status === 'stale'
                  ? 'Última listagem válida preservada'
                  : part.status === 'failed'
                    ? 'Listagem indisponível'
                    : 'Interfaces estáticas'}
                {part.observed_at ? ` · ${formatDate(part.observed_at)}` : ''}
              </p>
              {['failed', 'stale'].includes(part.status) ? (
                <p className="inventory-notice">
                  {equipmentMessage(part.code)}
                </p>
              ) : part.counts_known ? (
                <p className="muted">
                  {allRows.length} de {part.eligible} estáticas · {part.dynamic}{' '}
                  dinâmicas fora desta listagem · {part.total} no destino.{' '}
                  {part.status === 'limited'
                    ? 'Listagem limitada por volume ou tamanho: até 128 estáticas.'
                    : ''}
                </p>
              ) : (
                <p className="muted">
                  Contagens não registradas nesta leitura.
                </p>
              )}
              {part.status === 'stale' && (
                <details>
                  <summary>Origem da listagem preservada</summary>
                  <p className="fingerprint">{part.source}</p>
                </details>
              )}
            </>
          ) : (
            <p className="muted">
              Listagem da leitura anterior; escopo e contagens não registrados.
            </p>
          )}
          {!!rows.length && (
            <div
              className="table-scroll"
              tabIndex={0}
              aria-label="Interfaces observadas"
              ref={table}
            >
              <table className="app-table">
                <thead>
                  <tr>
                    <th>Nome</th>
                    <th>Tipo</th>
                    <th>Estado</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(page * 16, (page + 1) * 16).map((i) => (
                    <tr key={i.name}>
                      <td>{i.name}</td>
                      <td>{i.kind}</td>
                      <td>
                        {i.disabled
                          ? 'Desativada'
                          : i.running
                            ? 'Ativa'
                            : 'Parada'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {rows.length > 16 && (
            <div className="list-footer">
              <span>
                Página {page + 1} de {Math.ceil(rows.length / 16)}
              </span>
              <div>
                <Button disabled={!page} onClick={() => showPage(page - 1)}>
                  Anterior
                </Button>
                <Button
                  disabled={(page + 1) * 16 >= rows.length}
                  onClick={() => showPage(page + 1)}
                >
                  Próxima
                </Button>
              </div>
            </div>
          )}
          {!rows.length && part?.status !== 'failed' && (
            <p className="muted">Nenhuma interface nesta listagem.</p>
          )}
        </>
      )}
    </section>
  );
}
