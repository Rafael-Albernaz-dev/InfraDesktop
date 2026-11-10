import { useState } from 'react';
import { AccessScreen } from '../features/access/access-screen';
import type {
  PerformFn,
  useAccessSession,
} from '../features/access/access-session';
import { Statusbar } from './desktop';
import type { Intent } from '../features/access/microcopy';
import { access } from '../platform/wails/presentation';
import { Button } from '../shared/ui/Button';
function accessOperation(
  mode: string,
  name: string,
  password: string,
  code: string,
  remember: boolean,
) {
  return mode === 'register'
    ? access.register(name, password)
    : mode === 'recover'
      ? access.recover(name, code, password)
      : mode === 'reset'
        ? access.resetAccount(name, password)
        : access.login(name, password, remember);
}
export function UnauthenticatedShell(props: {
  theme: string;
  setTheme: (t: string) => void;
  session: ReturnType<typeof useAccessSession>;
  perform: PerformFn;
}) {
  const { theme, setTheme, session, perform } = props;
  const {
    view,
    busy,
    connected,
    error,
    recovery,
    setRecovery,
    invalidate,
    setError,
  } = session;
  return (
    <div className="app" data-theme={theme} data-route="access" data-page="1">
      <div className="client">
        <AccessScreen
          key={session.resetVerification ? 'verify-reset' : 'access'}
          resetVerification={session.resetVerification}
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
              'ack',
            )
          }
          contextChanged={() => {
            invalidate();
            setError('');
          }}
          submit={(mode, name, password, code, remember) =>
            void perform(
              () => accessOperation(mode, name, password, code, remember),
              undefined,
              true,
              mode as Intent,
            )
          }
        />
        {connected && <Statusbar connected />}
      </div>
    </div>
  );
}
export function CompanyForm({
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
        <Button className="btn primary" disabled={busy} type="submit">
          Criar empresa
        </Button>
      </div>
    </form>
  );
}
