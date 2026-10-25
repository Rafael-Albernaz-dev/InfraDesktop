import { useState } from 'react';
import type { FormEvent } from 'react';
import { RecoveryActions } from './recovery-actions';
import { Brand, Icon } from './icons';
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
export function AccessScreen({
  theme = 'light',
  setTheme,
  setup,
  busy,
  connected,
  error,
  recovery,
  acknowledge,
  contextChanged,
  initialName = '',
  initialMode = 'login',
  submit,
}: AccessProps) {
  const [mode, setMode] = useState(initialMode),
    [name, setName] = useState(initialName),
    [password, setPassword] = useState(''),
    [code, setCode] = useState(''),
    [remember, setRemember] = useState(true),
    [show, setShow] = useState(false);
  function change(next: string) {
    if ((busy && mode !== 'login') || recovery) return;
    contextChanged?.();
    setMode(next);
    setPassword('');
    setCode('');
    setShow(false);
  }
  function send(e: FormEvent) {
    e.preventDefault();
    if (busy || recovery) return;
    submit(mode, name, password, code, remember);
    setPassword('');
    setCode('');
  }
  return (
    <div className="auth">
      {setTheme && (
        <div
          className="access-theme"
          role="group"
          aria-label="Tema da interface"
        >
          {[
            ['light', 'Claro'],
            ['dark', 'Escuro'],
          ].map(([id, label]) => (
            <button
              type="button"
              key={id}
              className="link-btn"
              aria-pressed={theme === id}
              onClick={() => setTheme(id)}
            >
              {label}
            </button>
          ))}
        </div>
      )}
      <form className="auth-content" onSubmit={send}>
        <div className="auth-brand">
          <Brand />
        </div>
        {recovery ? (
          <>
            <h1>Guarde seu código</h1>
            <p>
              Este código aparece uma única vez. Guarde-o offline em local
              seguro para redefinir a senha.
            </p>
            <textarea
              className="recovery-code"
              aria-label="Código de recuperação"
              readOnly
              value={recovery}
              onFocus={(e) => e.currentTarget.select()}
            />
            <RecoveryActions busy={busy} />
            <p>
              Perder a senha e este código pode impedir o acesso à conta. Não há
              recuperação por e-mail.
            </p>
            <button
              type="button"
              className="btn primary"
              disabled={busy}
              onClick={() => {
                acknowledge();
                setMode('login');
                setPassword('');
                setCode('');
                setShow(false);
              }}
            >
              Guardei meu código
            </button>
            {error && (
              <p className="inline-error" role="alert">
                {error}
              </p>
            )}
          </>
        ) : (
          <>
            <h1>
              {mode === 'register'
                ? 'Crie sua conta local'
                : mode === 'recover'
                  ? 'Recuperar acesso'
                  : 'Entrar'}
            </h1>
            {mode !== 'login' && (
              <p>
                {mode === 'register'
                  ? 'Escolha o acesso desta instalação. A senha precisa ter de 15 a 128 caracteres.'
                  : 'Use seu código offline e escolha uma senha nova. Todos os acessos anteriores serão revogados.'}
              </p>
            )}
            <div className="field">
              <label htmlFor="access-user">Usuário</label>
              <input
                id="access-user"
                autoComplete="off"
                value={name}
                maxLength={80}
                required
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            {mode === 'recover' && (
              <div className="field">
                <label htmlFor="recovery-input">Código de recuperação</label>
                <input
                  id="recovery-input"
                  autoComplete="off"
                  required
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                />
              </div>
            )}
            <div className="field">
              <label htmlFor="access-password">
                {mode === 'recover' ? 'Nova senha' : 'Senha'}
              </label>
              <div className="password-wrap">
                <input
                  id="access-password"
                  type={show ? 'text' : 'password'}
                  autoComplete="off"
                  required
                  maxLength={128}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                {password && (
                  <button
                    type="button"
                    aria-label={show ? 'Ocultar senha' : 'Mostrar senha'}
                    onClick={() => setShow(!show)}
                  >
                    <Icon name="Eye" />
                  </button>
                )}
              </div>
            </div>
            {mode === 'login' && (
              <label className="check">
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                />
                Manter acesso nesta máquina
              </label>
            )}
            {error && (
              <p className="inline-error" role="alert">
                {error}
              </p>
            )}
            <button
              type="submit"
              className="btn primary"
              disabled={busy || !connected}
            >
              {busy
                ? 'Aguarde…'
                : mode === 'register'
                  ? 'Criar conta local'
                  : mode === 'recover'
                    ? 'Redefinir senha'
                    : 'Entrar'}
            </button>
            <div className="auth-links">
              <button
                type="button"
                className="link-btn"
                disabled={
                  (busy && mode !== 'login') || (mode === 'login' && !setup)
                }
                onClick={() => change(mode === 'login' ? 'register' : 'login')}
              >
                {mode === 'login' ? 'Criar conta' : 'Voltar ao login'}
              </button>
              {mode === 'login' && (
                <button
                  type="button"
                  className="link-btn"
                  onClick={() => change('recover')}
                >
                  Recuperar acesso
                </button>
              )}
            </div>
          </>
        )}
        {!connected && (
          <p role="status" className="inline-status">
            Aguardando serviço local
          </p>
        )}
      </form>
    </div>
  );
}
