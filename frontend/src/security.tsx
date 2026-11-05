import { useEffect, useState } from 'react';
import { Button } from './desktop-controls';
import { RecoveryActions } from './recovery-actions';

export function Security({
  recovery,
  busy,
  error,
  uncertain,
  lock,
  renew,
  acknowledge,
}: {
  recovery: string;
  busy: boolean;
  error: string;
  uncertain: boolean;
  lock: (locked: boolean) => void;
  renew: (password: string) => void;
  acknowledge: () => void;
}) {
  const [password, setPassword] = useState('');
  useEffect(() => {
    if (!busy && !recovery) lock(false);
  }, [uncertain, busy, recovery, lock]);
  return (
    <div className="security">
      <p>
        Se esquecer sua senha, este código permite criar uma nova. Gerar outro
        código invalida o anterior.
      </p>
      {recovery ? (
        <>
          <h3>Guarde seu novo código</h3>
          <p>
            Copiar não confirma que você o guardou em um local seguro. Confirme
            ao terminar.
          </p>
          <label htmlFor="new-recovery">Novo código de recuperação</label>
          <textarea
            id="new-recovery"
            className="recovery-code"
            readOnly
            value={recovery}
            onFocus={(event) => event.currentTarget.select()}
          />
          <RecoveryActions busy={busy} />
          <div className="dialog-actions">
            <Button kind="primary" disabled={busy} onClick={acknowledge}>
              Guardei meu código
            </Button>
          </div>
        </>
      ) : (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (busy) return;
            lock(true);
            const value = password;
            setPassword('');
            renew(value);
          }}
        >
          <div className="field">
            <label htmlFor="security-password">Senha atual</label>
            <input
              id="security-password"
              autoComplete="current-password"
              type="password"
              maxLength={128}
              required
              disabled={busy}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
          {uncertain && !busy && (
            <p>
              Não recebemos a confirmação. O código anterior pode ter sido
              invalidado. Confirme a senha para gerar outro.
            </p>
          )}
          <div className="dialog-actions">
            <Button className="btn primary" type="submit" disabled={busy}>
              {uncertain ? 'Gerar outro código novo' : 'Gerar novo código'}
            </Button>
          </div>
        </form>
      )}
      {error && (
        <p
          role="alert"
          className={
            error === 'SENHA INCORRETA'
              ? 'inline-error inline-error-danger'
              : 'inline-error'
          }
        >
          {error}
        </p>
      )}
    </div>
  );
}
