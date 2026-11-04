import { RecoveryCode } from './ui/RecoveryCode';
import { useState } from 'react';
import type { FormEvent } from 'react';
import { RecoveryActions } from './recovery-actions';
import { Button } from './ui/Button';
import { AccessForm } from './access-fields';
import { ProductBrand } from './ui/ProductBrand';
export type AccessProps = {
  theme?: string;
  setTheme?: (theme: string) => void;
  setup: boolean;
  busy: boolean;
  connected: boolean;
  error: string;
  recovery: string;
  acknowledge: () => void;
  contextChanged?: () => void;
  initialName?: string;
  initialMode?: string;
  submit: (
    mode: string,
    name: string,
    password: string,
    code: string,
    remember: boolean,
  ) => void;
};

export function useAccessFields(p: AccessProps) {
  const [mode, setMode] = useState(p.initialMode || 'login');
  const [name, setName] = useState(p.initialName || '');
  const [password, setPassword] = useState(''),
    [confirm, setConfirm] = useState('');
  const [code, setCode] = useState(''),
    [remember, setRemember] = useState(true);
  const [show, setShow] = useState(false),
    [localError, setLocalError] = useState('');
  function change(next: string) {
    if ((p.busy && mode !== 'login') || p.recovery) return;
    p.contextChanged?.();
    setMode(next);
    setPassword('');
    setConfirm('');
    setCode('');
    setShow(false);
    setLocalError('');
  }
  function send(event: FormEvent) {
    event.preventDefault();
    if (p.busy || p.recovery) return;
    if (mode === 'register' && password !== confirm) {
      setLocalError('As senhas não conferem.');
      return;
    }
    p.submit(mode, name, password, code, remember);
    setPassword('');
    setConfirm('');
    setCode('');
    setLocalError('');
  }
  return {
    mode,
    name,
    setName,
    password,
    setPassword,
    confirm,
    setConfirm,
    code,
    setCode,
    remember,
    setRemember,
    show,
    setShow,
    localError,
    finish: () => {
      setMode('login');
      setPassword('');
      setConfirm('');
      setCode('');
      setShow(false);
    },
    change,
    send,
  };
}

export function AccessScreen(p: AccessProps) {
  const s = useAccessFields(p);
  const title = p.recovery
    ? 'Guarde seu novo código'
    : s.mode === 'register'
      ? 'Criar conta'
      : s.mode === 'recover'
        ? 'Recuperar acesso'
        : 'Entrar';
  const subtitle = p.recovery
    ? 'Ele aparece só agora e recupera sua conta.'
    : s.mode === 'register'
      ? 'A conta fica só neste computador.'
      : s.mode === 'recover'
        ? 'Use seu código e escolha uma senha nova.'
        : 'Use a conta local deste computador.';
  return (
    <div data-theme={p.theme || 'light'}>
      {p.setTheme && (
        <div className="tema" role="group" aria-label="Tema da interface">
          {[
            ['light', 'Claro'],
            ['dark', 'Escuro'],
          ].map(([theme, text]) => (
            <Button
              key={theme}
              aria-pressed={(p.theme || 'light') === theme}
              onClick={() => p.setTheme?.(theme)}
            >
              {text}
            </Button>
          ))}
        </div>
      )}
      <div className="login">
        <div className="lg-card">
          <div className="lg-brand">
            <ProductBrand />
          </div>
          <div className="lg-corpo">
            <h1 className="lg-title">{title}</h1>
            <p className="lg-sub">{subtitle}</p>
            {p.recovery ? (
              <>
                <RecoveryCode value={p.recovery} />
                <RecoveryActions busy={p.busy} />
                <Button
                  className="lg-btn"
                  disabled={p.busy}
                  onClick={() => {
                    p.acknowledge();
                    s.finish();
                  }}
                >
                  Guardei meu código
                </Button>
              </>
            ) : (
              <AccessForm s={s} p={p} />
            )}
            {(p.error || s.localError) && (
              <p role="alert">{p.error || s.localError}</p>
            )}
            {!p.connected && <p role="status">Aguardando serviço local</p>}
          </div>
        </div>
      </div>
    </div>
  );
}
