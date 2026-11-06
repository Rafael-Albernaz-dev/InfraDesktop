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
  busy: boolean;
  error: string;
  recoveryUncertain: boolean;
  securityLockedRef: React.MutableRefObject<boolean>;
  workingRef: React.MutableRefObject<boolean>;
  perform: PerformFn;
  setRecovery: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  close: () => void;
}) {
  const {
    recovery,
    busy,
    error,
    recoveryUncertain,
    securityLockedRef,
    workingRef,
    perform,
    setRecovery,
    setRecoveryUncertain,
    close,
  } = props;
  return (
    <Security
      recovery={recovery}
      busy={busy}
      error={error}
      uncertain={recoveryUncertain}
      cancel={close}
      lock={(locked) => {
        securityLockedRef.current = locked;
      }}
      renew={(password) => {
        if (workingRef.current) return;
        const replace = recoveryUncertain;
        securityLockedRef.current = true;
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
            securityLockedRef.current = false;
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
  busy: boolean;
  error: string;
  recoveryUncertain: boolean;
  securityLockedRef: React.MutableRefObject<boolean>;
  workingRef: React.MutableRefObject<boolean>;
  perform: PerformFn;
  setRecovery: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  setPage: (p: Page) => void;
}) {
  const { dialog, close, busy, error, perform, setPage } = props;
  if (!dialog) return null;
  return (
    <Dialog
      title={dialog === 'Segurança' ? (props.recovery ? 'Guarde seu código' : 'Gerar novo código') : dialog}
      close={close}
      showClose={dialog !== 'Segurança'}
    >
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
  securityLockedRef: React.MutableRefObject<boolean>;
  epochRef: React.MutableRefObject<number>;
  hadAuthRef: React.MutableRefObject<boolean>;
  contextKeyRef: React.MutableRefObject<string>;
  surfaceRef: React.MutableRefObject<number>;
  previousViewRef: React.MutableRefObject<Result>;
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
  const previous = refs.previousViewRef.current;
  if (refs.contextKeyRef.current && refs.contextKeyRef.current !== key)
    refs.epochRef.current++;
  if (hasScopeChanged(previous, r)) {
    refs.setError('');
    refs.surfaceRef.current++;
    if (!r.authenticated && previous.authenticated) {
      refs.securityLockedRef.current = false;
      refs.setRecoveryUncertain(false);
      refs.setRecovery('');
    }
    if (!refs.securityLockedRef.current) {
      refs.setDialog('');
      if (previous.authenticated || r.authenticated) refs.setRecovery('');
    }
  }
  refs.previousViewRef.current = r;
  refs.contextKeyRef.current = key;
  refs.setView({ ...r, recovery: '' });
  refs.setConnected(r.code === 'OK');
  if (r.authenticated) {
    setTheme(r.theme);
    if (!refs.hadAuthRef.current) setPage('companies');
  }
  refs.hadAuthRef.current = r.authenticated;
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
  const [resetVerification, setResetVerification] = useState(false);
  const [dialog, setDialog] = useState('');
  const [recoveryUncertain, setRecoveryUncertain] = useState(false);
  const securityLockedRef = useRef(false);
  const epochRef = useRef(0);
  const workingRef = useRef(false);
  const hadAuthRef = useRef(false);
  const contextKeyRef = useRef('');
  const surfaceRef = useRef(0);
  const previousViewRef = useRef(initial);
  const bumpSurface = useCallback(() => {
    surfaceRef.current++;
  }, []);
  const accept = useCallback(
    (r: Result) => {
      applySessionResult(
        r,
        {
          securityLockedRef,
          epochRef,
          hadAuthRef,
          contextKeyRef,
          surfaceRef,
          previousViewRef,
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
    epochRef.current++;
  }, []);
  useStatusPoll(workingRef, epochRef, accept, setConnected, invalidate);
  return {
    view,
    connected,
    busy,
    setBusy,
    error,
    setError,
    recovery,
    setRecovery,
    resetVerification,
    setResetVerification,
    dialog,
    setDialog,
    recoveryUncertain,
    setRecoveryUncertain,
    securityLockedRef,
    epochRef,
    workingRef,
    surfaceRef,
    bumpSurface,
    accept,
    invalidate,
  };
}

export { createPerformAction } from './app-perform';

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
