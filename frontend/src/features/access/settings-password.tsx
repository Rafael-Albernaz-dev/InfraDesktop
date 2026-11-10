import type { FormEvent } from 'react';
import { useState } from 'react';
import { AccessError } from './access-error';
import type { DesktopProps } from './desktop-model';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

export function SettingsPassword({ p }: { p: DesktopProps }) {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [localError, setLocalError] = useState('');
  const [success, setSuccess] = useState(false);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (p.busy || !p.connected || !p.changePassword) return;
    if (next !== confirm) {
      setLocalError('As senhas não conferem.');
      return;
    }
    setLocalError('');
    setSuccess(false);
    const result = p.changePassword(current, next);
    setCurrent('');
    setNext('');
    setConfirm('');
    if (await result) setSuccess(true);
  };
  return (
    <form
      className="card settings-card"
      onSubmit={(event) => void submit(event)}
    >
      <h2>Senha</h2>
      <Field
        layout="form"
        label="Senha atual"
        type="password"
        autoComplete="current-password"
        required
        maxLength={128}
        value={current}
        onChange={(e) => setCurrent(e.target.value)}
      />
      <Field
        layout="form"
        label="Nova senha"
        type="password"
        autoComplete="new-password"
        required
        minLength={8}
        maxLength={128}
        value={next}
        onChange={(e) => setNext(e.target.value)}
      />
      <Field
        layout="form"
        label="Confirmar nova senha"
        type="password"
        autoComplete="new-password"
        required
        minLength={8}
        maxLength={128}
        value={confirm}
        onChange={(e) => setConfirm(e.target.value)}
      />
      {(localError || p.passwordError) && (
        <AccessError
          text={localError || p.passwordError || ''}
          kind={localError ? 'general' : (p.passwordErrorKind ?? 'general')}
        />
      )}
      {success && <p role="status">Senha alterada.</p>}
      <Button
        kind="primary"
        type="submit"
        disabled={p.busy || !p.connected || !p.changePassword}
      >
        Alterar senha
      </Button>
    </form>
  );
}
