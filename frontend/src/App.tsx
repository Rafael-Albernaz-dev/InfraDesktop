import { useCallback, useEffect, useRef, useState } from 'react';
import { access } from './presentation';
import type { Result } from './presentation';
import { Desktop, Dialog, Button, Statusbar } from './desktop';
import type { Page } from './desktop';
import { AccessScreen } from './access-screen';
import './style.css';
import { message } from './microcopy';
import type { Intent } from './microcopy';
import type { Branding } from './desktop-model';
import { Security } from './security';
import { Equipment } from './equipment';
import { useEquipmentOperations } from './equipment-operations';
const initial: Result = {
  code: 'SERVICE_UNAVAILABLE',
  request_id: '',
  instance_id: '',
  activation_id: '',
  profile_id: '',
  heartbeat: 0,
  setup: false,
  authenticated: false,
  name: '',
  theme: 'light',
  recovery: '',
  companies: [],
};
export default function App() {
  const [view, setView] = useState(initial),
    [theme, setTheme] = useState('light'),
    [page, setPage] = useState<Page>(2);
  const [inventoryAction, addInventory] = useState(0);
  const [inventoryTarget, setInventoryTarget] = useState<{
    key: string;
    id: string;
  }>({ key: '', id: '' });
  const [inventoryDetail, setInventoryDetail] = useState(false);
  const [inventoryState, setInventoryState] = useState({ key: '', total: 0 });
  const reportInventory = useCallback((key: string, total: number) => {
    setInventoryState((old) =>
      old.key === key && old.total === total ? old : { key, total },
    );
  }, []);
  const [connected, setConnected] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [recovery, setRecovery] = useState(''),
    [dialog, setDialog] = useState('');
  const [brandState, setBrandState] = useState<{
    key: string;
    value: Branding;
  }>({ key: '', value: { name: '', logo: '', accent: 'teal' } });
  const brandRead = useRef(0);
  const [brandRetry, retryBrand] = useState(0);
  const [brandError, setBrandError] = useState('');
  const operations = useEquipmentOperations(
    view.profile_id,
    view.activation_id,
    connected && view.authenticated,
    page === 3,
  );
  const selectInventory = useCallback(
    (id: string) =>
      setInventoryTarget({
        key: [view.profile_id, view.activation_id].join('|'),
        id,
      }),
    [view.profile_id, view.activation_id],
  );
  const [recoveryUncertain, setRecoveryUncertain] = useState(false);
  const securityLocked = useRef(false);
  const epoch = useRef(0),
    working = useRef(false),
    hadAuth = useRef(false),
    contextKey = useRef(''),
    surface = useRef(0),
    previousView = useRef(initial);
  const accept = useCallback((r: Result) => {
    const key = [
      r.instance_id,
      r.authenticated,
      r.profile_id,
      r.activation_id,
      r.name,
    ].join('|');
    const previous = previousView.current;
    const reconnected = previous.instance_id !== r.instance_id;
    const scopeChanged =
      previous.authenticated !== r.authenticated ||
      previous.profile_id !== r.profile_id ||
      previous.name !== r.name ||
      (!reconnected && previous.activation_id !== r.activation_id);
    if (contextKey.current && contextKey.current !== key) epoch.current++;
    // A restarted transport invalidates pending responses, not actionable
    // feedback or the one-time recovery code awaiting human acknowledgement.
    if (scopeChanged) {
      setError('');
      surface.current++;
      if (!r.authenticated && previous.authenticated) {
        securityLocked.current = false;
        setRecoveryUncertain(false);
        setRecovery('');
      }
      if (!securityLocked.current) {
        setDialog('');
        if (previous.authenticated || r.authenticated) setRecovery('');
      }
    }
    previousView.current = r;
    contextKey.current = key;
    setView({ ...r, recovery: '' });
    setConnected(r.code === 'OK');
    if (r.authenticated) {
      setTheme(r.theme);
      if (!hadAuth.current) setPage(r.profile_id ? 3 : 2);
    }
    hadAuth.current = r.authenticated;
  }, []);
  const invalidate = useCallback(() => {
    epoch.current++;
  }, []);
  useEffect(() => {
    let stopped = false,
      timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    const poll = async () => {
      const generation = epoch.current;
      if (!working.current) {
        try {
          const r = await access.status();
          if (!stopped && generation === epoch.current) {
            if (r.code === 'OK') {
              failures = 0;
              accept(r);
            } else {
              failures++;
              setConnected(false);
              if (r.code === 'UNAUTHENTICATED') accept(r);
            }
          }
        } catch {
          if (!stopped && generation === epoch.current) {
            failures++;
            setConnected(false);
          }
        }
      }
      if (!stopped)
        timer = setTimeout(
          () => void poll(),
          Math.min(10000, 1000 * 2 ** Math.min(failures, 4)),
        );
    };
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
      invalidate();
    };
  }, [accept, invalidate]);
  async function perform(
    call: () => Promise<Result>,
    after?: (r: Result) => void,
    applyResult = true,
    intent: Intent = 'general',
  ) {
    if (working.current) return false;
    working.current = true;
    setBusy(true);
    setError('');
    const generation = ++epoch.current;
    const operationSurface = surface.current;
    try {
      const r = await call();
      if (generation !== epoch.current) return false;
      if (intent === 'renew') {
        setRecoveryUncertain((previous) =>
          r.code === 'OK'
            ? false
            : previous ||
              ['DEADLINE_EXCEEDED', 'SERVICE_UNAVAILABLE'].includes(r.code),
        );
        if (r.code !== 'OK') securityLocked.current = false;
      }
      if (r.code !== 'OK') {
        if (operationSurface === surface.current)
          setError(message(r.code, intent));
        return false;
      }
      const ownsSurface = operationSurface === surface.current;
      if (applyResult) accept(r);
      if (r.recovery)
        setRecovery(r.recovery.match(/.{1,8}/g)?.join('-') || r.recovery);
      if (ownsSurface) after?.(r);
      return true;
    } catch {
      if (intent === 'renew') {
        setRecoveryUncertain(true);
        securityLocked.current = false;
      }
      if (generation === epoch.current && operationSurface === surface.current)
        setError(message('SERVICE_UNAVAILABLE', intent));
      return false;
    } finally {
      working.current = false;
      setBusy(false);
    }
  }
  const company = (view.companies || []).find((c) => c.id === view.profile_id);
  const brandKey = [
    view.instance_id,
    view.profile_id,
    view.activation_id,
    view.authenticated,
  ].join('|');
  useEffect(() => {
    let cancelled = false;
    const read = ++brandRead.current;
    if (!view.authenticated || !view.profile_id || !connected) return;
    void access
      .getBrand(view.profile_id, view.activation_id)
      .then((r) => {
        if (cancelled || read !== brandRead.current) return;
        if (r.code === 'OK' && r.brand) {
          setBrandState({ key: brandKey, value: r.brand });
          setBrandError('');
        } else setBrandError(message(r.code, 'brand'));
      })
      .catch(() => {
        if (!cancelled && read === brandRead.current)
          setBrandError(message('SERVICE_UNAVAILABLE', 'brand'));
      });
    return () => {
      cancelled = true;
    };
  }, [
    brandKey,
    view.authenticated,
    view.profile_id,
    view.activation_id,
    connected,
    brandRetry,
  ]);
  const savedBrand =
    brandState.key === brandKey
      ? brandState.value
      : { name: '', logo: '', accent: 'teal' };

  const open = (title: string) => {
    if (securityLocked.current) return;
    surface.current++;
    setError('');
    setDialog(title);
  };
  const close = () => open('');
  const overlays = dialog ? (
    <Dialog title={dialog} close={close}>
      {dialog === 'Segurança' ? (
        <Security
          recovery={recovery}
          theme={theme}
          setTheme={(next) => void perform(() => access.setTheme(next))}
          busy={busy}
          error={error}
          uncertain={recoveryUncertain}
          lock={(locked) => {
            securityLocked.current = locked;
          }}
          renew={(password) => {
            if (working.current) return;
            const replace = recoveryUncertain;
            securityLocked.current = true;
            void perform(
              () => access.renewRecovery(password, replace),
              undefined,
              true,
              'renew',
            );
          }}
          acknowledge={() =>
            void perform(
              () => access.ackRecovery(),
              () => {
                setRecovery('');
                setRecoveryUncertain(false);
                securityLocked.current = false;
                close();
              },
            )
          }
        />
      ) : dialog === 'Nova empresa' ? (
        <CompanyForm
          busy={busy}
          error={error}
          save={(name) =>
            void perform(
              () => access.createCompany(name),
              close,
              true,
              'company',
            )
          }
        />
      ) : dialog === 'Sair desta instalação?' ? (
        <>
          <p>
            Sair revoga os acessos lembrados desta conta. O serviço e os dados
            locais permanecem.
          </p>
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button onClick={close}>Cancelar</Button>
            <Button
              kind="primary"
              disabled={busy}
              onClick={() =>
                void perform(
                  () => access.logout(),
                  () => {
                    close();
                    setPage(2);
                  },
                )
              }
            >
              Sair
            </Button>
          </div>
        </>
      ) : (
        <>
          <p>
            Esta capacidade ainda não está disponível. Nenhuma consulta de
            equipamento foi realizada.
          </p>
          <div className="dialog-actions">
            <Button onClick={close}>Fechar</Button>
          </div>
        </>
      )}
    </Dialog>
  ) : null;
  if (!view.authenticated)
    return (
      <div className="app" data-theme={theme} data-page="1">
        <div className="client">
          <AccessScreen
            theme={theme}
            setTheme={setTheme}
            setup={view.setup}
            busy={busy}
            connected={connected}
            error={error}
            recovery={recovery}
            acknowledge={() =>
              void perform(
                () => access.ackRecovery(),
                () => setRecovery(''),
                false,
              )
            }
            contextChanged={() => {
              invalidate();
              setError('');
            }}
            submit={(mode, name, password, code, remember) =>
              void perform(
                () =>
                  mode === 'register'
                    ? access.register(name, password)
                    : mode === 'recover'
                      ? access.recover(name, code, password)
                      : access.login(name, password, remember),
                undefined,
                true,
                mode as Intent,
              )
            }
          />
          <Statusbar connected={connected} access />
        </div>
      </div>
    );
  return (
    <Desktop
      operations={operations}
      inventoryDetailOpen={inventoryDetail}
      openEquipment={(id) => {
        selectInventory(id);
        setPage(4);
      }}
      inventoryHasDevices={
        inventoryState.key ===
          [view.profile_id, view.activation_id].join('|') &&
        inventoryState.total > 0
      }
      inventory={
        <Equipment
          key={brandKey}
          profile={view.profile_id}
          activation={view.activation_id}
          connected={connected}
          createRequest={inventoryAction}
          primaryInHeader
          targetID={
            inventoryTarget.key ===
            [view.profile_id, view.activation_id].join('|')
              ? inventoryTarget.id
              : undefined
          }
          onSelect={selectInventory}
          onDetailChange={setInventoryDetail}
          onTotal={reportInventory}
        />
      }
      addEquipment={() => {
        setPage(4);
        addInventory((n) => n + 1);
      }}
      page={page}
      navigate={(next) => {
        if (securityLocked.current) return;
        invalidate();
        addInventory(0);
        setPage(next);
        close();
      }}
      brand={{ ...savedBrand, name: savedBrand.name || company?.name || '' }}
      savedBrand={savedBrand}
      brandError={brandError}
      retryBrand={() => retryBrand((n) => n + 1)}
      saveBrand={
        view.profile_id && brandState.key === brandKey
          ? async (brand) => {
              ++brandRead.current;
              const expected = brandKey;
              let success = false;
              let committed = false;
              await perform(
                async () => {
                  const result = await access.saveBrand(
                    view.profile_id,
                    view.activation_id,
                    brand,
                  );
                  committed = result.code === 'OK';
                  return result;
                },
                (r) => {
                  if (r.brand && expected === brandKey) {
                    setBrandState({ key: expected, value: r.brand });
                    setBrandError('');
                    success = true;
                  }
                },
                false,
                'brand',
              );
              if (committed && !success) retryBrand((n) => n + 1);
              return success;
            }
          : undefined
      }
      account={view.name}
      theme={theme}
      setTheme={(next) => void perform(() => access.setTheme(next))}
      connected={connected}
      busy={busy}
      companies={view.companies || []}
      activeProfile={view.profile_id}
      create={() => open('Nova empresa')}
      enter={(id) =>
        void perform(
          () => access.selectCompany(id),
          () => setPage(3),
        )
      }
      security={() => open('Segurança')}
      logout={() => open('Sair desta instalação?')}
      unavailable={(name) => open(name + ' indisponível')}
      error={dialog ? '' : error}
    >
      {overlays}
    </Desktop>
  );
}
function CompanyForm({
  busy,
  error,
  save,
}: {
  busy: boolean;
  error: string;
  save: (name: string) => void;
}) {
  const [name, setName] = useState('');
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        save(name);
      }}
    >
      <div className="field">
        <label htmlFor="company-name">Nome da empresa</label>
        <input
          id="company-name"
          required
          maxLength={80}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>
      {error && (
        <p role="alert" className="inline-error">
          {error}
        </p>
      )}
      <div className="dialog-actions">
        <button className="btn primary" disabled={busy} type="submit">
          Criar empresa
        </button>
      </div>
    </form>
  );
}
