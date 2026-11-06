import type { useAccessSession } from './access-session';
import { PerformFn } from './access-session';
import { CompanyForm } from './app-dialogs';
import type { Page } from './desktop';
import { Button, Dialog } from './desktop';
import { access } from './presentation';
import { SecurityDialogBody } from './security-dialog-body';
import type { SecurityDialogBodyProps } from './security-dialog-body';

type AppOverlaysProps = SecurityDialogBodyProps & {
  dialog: string;
  setPage: (p: Page) => void;
};
export function AppOverlays(props: AppOverlaysProps) {
  const { dialog, close, busy, error, perform, setPage } = props;
  if (!dialog) return null;
  return (
    <Dialog
      title={
        dialog === 'Segurança'
          ? props.recovery
            ? 'Guarde seu código'
            : 'Gerar novo código'
          : dialog
      }
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

type LogoutDialogBodyProps = {
  busy: boolean;
  error: string;
  close: () => void;
  perform: PerformFn;
  setPage: (p: Page) => void;
};
function LogoutDialogBody(props: LogoutDialogBodyProps) {
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

export { CompanyForm, UnauthenticatedShell } from './app-dialogs';

type AuthenticatedOverlaysProps = {
  session: ReturnType<typeof useAccessSession>;
  perform: PerformFn;
  close: () => void;
  setPage: (p: Page) => void;
};
export function AuthenticatedOverlays(props: AuthenticatedOverlaysProps) {
  const { session, perform, close, setPage } = props;
  return (
    <AppOverlays
      dialog={session.dialog}
      close={close}
      recovery={session.recovery}
      busy={session.busy}
      error={session.error}
      errorKind={session.errorKind}
      recoveryUncertain={session.recoveryUncertain}
      securityLockedRef={session.securityLockedRef}
      workingRef={session.workingRef}
      perform={perform}
      setRecovery={session.setRecovery}
      setRecoveryUncertain={session.setRecoveryUncertain}
      setPage={setPage}
    />
  );
}
