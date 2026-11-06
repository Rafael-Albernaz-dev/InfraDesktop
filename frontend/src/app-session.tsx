import { CompanyForm } from './app-dialogs';
import { PerformFn } from './access-session';
import type { Page } from './desktop';
import { Button,Dialog } from './desktop';
import { access } from './presentation';
import { Security } from './security';

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

export { CompanyForm,UnauthenticatedShell } from './app-dialogs';
