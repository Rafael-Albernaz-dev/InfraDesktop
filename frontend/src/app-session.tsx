import { acceptEquipmentAuthority } from './equipment-receipts';
import { useCallback, useEffect, useRef, useState } from 'react';
import { access } from './presentation';
import type { Result } from './presentation';
import { Dialog, Button } from './desktop';
import type { Page } from './desktop';
import type { Branding } from './desktop-model';
import { message } from './microcopy';
import type { Intent } from './microcopy';
import { Security } from './security';
import { CompanyForm } from './app-dialogs';

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

export function useBrandState(view: Result, connected: boolean) {
  const [brandState, setBrandState] = useState<{
    key: string;
    value: Branding;
  }>({ key: '', value: { name: '', logo: '', accent: 'teal' } });
  const brandRead = useRef(0);
  const [brandRetry, retryBrand] = useState(0);
  const [brandError, setBrandError] = useState('');
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
  return {
    brandKey,
    brandState,
    setBrandState,
    brandRead,
    brandError,
    setBrandError,
    retryBrand,
    savedBrand,
  };
}

function useStatusPoll(
  working: React.RefObject<boolean>,
  epoch: React.RefObject<number>,
  accept: (r: Result) => void,
  setConnected: (v: boolean) => void,
  invalidate: () => void,
) {
  useEffect(() => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
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
  }, [accept, epoch, invalidate, setConnected, working]);
}

export type PerformFn = (
  call: () => Promise<Result>,
  after?: (r: Result) => void,
  applyResult?: boolean,
  intent?: Intent,
) => Promise<boolean>;

function SecurityDialogBody(props: {
  recovery: string;
  theme: string;
  busy: boolean;
  error: string;
  recoveryUncertain: boolean;
  securityLocked: React.MutableRefObject<boolean>;
  working: React.MutableRefObject<boolean>;
  perform: PerformFn;
  setRecovery: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  close: () => void;
}) {
  const {
    recovery,
    theme,
    busy,
    error,
    recoveryUncertain,
    securityLocked,
    working,
    perform,
    setRecovery,
    setRecoveryUncertain,
    close,
  } = props;
  return (
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
  );
}

export function AppOverlays(props: {
  dialog: string;
  close: () => void;
  recovery: string;
  theme: string;
  busy: boolean;
  error: string;
  recoveryUncertain: boolean;
  securityLocked: React.MutableRefObject<boolean>;
  working: React.MutableRefObject<boolean>;
  perform: PerformFn;
  setRecovery: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  setPage: (p: Page) => void;
}) {
  const { dialog, close, busy, error, perform, setPage } = props;
  if (!dialog) return null;
  return (
    <Dialog title={dialog} close={close}>
      {dialog === 'Segurança' ? (
        <SecurityDialogBody {...props} />
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
        <LogoutDialogBody
          busy={busy}
          error={error}
          close={close}
          perform={perform}
          setPage={setPage}
        />
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
  );
}

function LogoutDialogBody(props: {
  busy: boolean;
  error: string;
  close: () => void;
  perform: PerformFn;
  setPage: (p: Page) => void;
}) {
  const { busy, error, close, perform, setPage } = props;
  return (
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
                setPage('companies');
              },
            )
          }
        >
          Sair
        </Button>
      </div>
    </>
  );
}

function syncAuthority(r: Result) {
  if (r.code === 'UNAUTHENTICATED') acceptEquipmentAuthority('', '');
  else if (r.code === 'OK')
    acceptEquipmentAuthority(
      r.authenticated ? r.profile_id : '',
      r.authenticated ? r.activation_id : '',
    );
}

function hasScopeChanged(previous: Result, r: Result): boolean {
  const reconnected = previous.instance_id !== r.instance_id;
  return (
    previous.authenticated !== r.authenticated ||
    previous.profile_id !== r.profile_id ||
    previous.name !== r.name ||
    (!reconnected && previous.activation_id !== r.activation_id)
  );
}

type SessionRefs = {
  securityLocked: React.MutableRefObject<boolean>;
  epoch: React.MutableRefObject<number>;
  hadAuth: React.MutableRefObject<boolean>;
  contextKey: React.MutableRefObject<string>;
  surface: React.MutableRefObject<number>;
  previousView: React.MutableRefObject<Result>;
  setError: (v: string) => void;
  setRecovery: (v: string) => void;
  setDialog: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  setView: (v: Result) => void;
  setConnected: (v: boolean) => void;
};

function applySessionResult(
  r: Result,
  refs: SessionRefs,
  setPage: (p: Page) => void,
  setTheme: (t: string) => void,
) {
  syncAuthority(r);
  const key = [
    r.instance_id,
    r.authenticated,
    r.profile_id,
    r.activation_id,
    r.name,
  ].join('|');
  const previous = refs.previousView.current;
  if (refs.contextKey.current && refs.contextKey.current !== key)
    refs.epoch.current++;
  if (hasScopeChanged(previous, r)) {
    refs.setError('');
    refs.surface.current++;
    if (!r.authenticated && previous.authenticated) {
      refs.securityLocked.current = false;
      refs.setRecoveryUncertain(false);
      refs.setRecovery('');
    }
    if (!refs.securityLocked.current) {
      refs.setDialog('');
      if (previous.authenticated || r.authenticated) refs.setRecovery('');
    }
  }
  refs.previousView.current = r;
  refs.contextKey.current = key;
  refs.setView({ ...r, recovery: '' });
  refs.setConnected(r.code === 'OK');
  if (r.authenticated) {
    setTheme(r.theme);
    if (!refs.hadAuth.current) setPage(r.profile_id ? 'overview' : 'companies');
  }
  refs.hadAuth.current = r.authenticated;
}

export function useAccessSession(
  setPage: (p: Page) => void,
  setTheme: (t: string) => void,
) {
  const [view, setView] = useState(initial);
  const [connected, setConnected] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [recovery, setRecovery] = useState('');
  const [dialog, setDialog] = useState('');
  const [recoveryUncertain, setRecoveryUncertain] = useState(false);
  const securityLocked = useRef(false);
  const epoch = useRef(0);
  const working = useRef(false);
  const hadAuth = useRef(false);
  const contextKey = useRef('');
  const surface = useRef(0);
  const previousView = useRef(initial);
  const accept = useCallback(
    (r: Result) => {
      applySessionResult(
        r,
        {
          securityLocked,
          epoch,
          hadAuth,
          contextKey,
          surface,
          previousView,
          setError,
          setRecovery,
          setDialog,
          setRecoveryUncertain,
          setView,
          setConnected,
        },
        setPage,
        setTheme,
      );
    },
    [setPage, setTheme],
  );
  const invalidate = useCallback(() => {
    epoch.current++;
  }, []);
  useStatusPoll(working, epoch, accept, setConnected, invalidate);
  return {
    view,
    connected,
    busy,
    setBusy,
    error,
    setError,
    recovery,
    setRecovery,
    dialog,
    setDialog,
    recoveryUncertain,
    setRecoveryUncertain,
    securityLocked,
    epoch,
    working,
    surface,
    accept,
    invalidate,
  };
}

export function createPerformAction(
  session: ReturnType<typeof useAccessSession>,
): PerformFn {
  return async function perform(
    call: () => Promise<Result>,
    after?: (r: Result) => void,
    applyResult = true,
    intent: Intent = 'general',
  ) {
    if (session.working.current) return false;
    session.working.current = true;
    session.setBusy(true);
    session.setError('');
    const generation = ++session.epoch.current;
    const operationSurface = session.surface.current;
    try {
      const r = await call();
      if (generation !== session.epoch.current) return false;
      if (intent === 'renew') {
        session.setRecoveryUncertain((prev) =>
          r.code === 'OK'
            ? false
            : prev ||
              ['DEADLINE_EXCEEDED', 'SERVICE_UNAVAILABLE'].includes(r.code),
        );
        if (r.code !== 'OK') session.securityLocked.current = false;
      }
      if (r.code !== 'OK') {
        if (operationSurface === session.surface.current)
          session.setError(message(r.code, intent));
        return false;
      }
      const ownsSurface = operationSurface === session.surface.current;
      if (applyResult) session.accept(r);
      if (r.recovery)
        session.setRecovery(
          r.recovery.match(/.{1,8}/g)?.join('-') || r.recovery,
        );
      if (ownsSurface) after?.(r);
      return true;
    } catch {
      if (intent === 'renew') {
        session.setRecoveryUncertain(true);
        session.securityLocked.current = false;
      }
      if (
        generation === session.epoch.current &&
        operationSurface === session.surface.current
      )
        session.setError(message('SERVICE_UNAVAILABLE', intent));
      return false;
    } finally {
      session.working.current = false;
      session.setBusy(false);
    }
  };
}

export function createBrandSaver(
  view: Result,
  brand: ReturnType<typeof useBrandState>,
  perform: PerformFn,
) {
  if (!view.profile_id || brand.brandState.key !== brand.brandKey)
    return undefined;
  return async (nextBrand: Branding) => {
    ++brand.brandRead.current;
    const expected = brand.brandKey;
    let success = false;
    let committed = false;
    await perform(
      async () => {
        const result = await access.saveBrand(
          view.profile_id,
          view.activation_id,
          nextBrand,
        );
        committed = result.code === 'OK';
        return result;
      },
      (r) => {
        if (r.brand && expected === brand.brandKey) {
          brand.setBrandState({ key: expected, value: r.brand });
          brand.setBrandError('');
          success = true;
        }
      },
      false,
      'brand',
    );
    if (committed && !success) brand.retryBrand((n) => n + 1);
    return success;
  };
}

export { CompanyForm, UnauthenticatedShell } from './app-dialogs';
