import type { AccessProps, useAccessFields } from './use-access-fields';
import { Button } from './ui/Button';
import { Field } from './ui/Field';
type Fields = ReturnType<typeof useAccessFields>;
function PasswordField({ s }: { s: Fields }) {
  return (
    <label className="lg-field">
      <span>Senha</span>
      <span className="lg-pass">
        <input
          type={s.show ? 'text' : 'password'}
          aria-label="Senha"
          autoComplete="current-password"
          placeholder="••••••••"
          required
          maxLength={128}
          value={s.password}
          onChange={(e) => s.setPassword(e.target.value)}
        />
        <Button
          className="lg-eye"
          label={s.show ? 'Ocultar senha' : 'Mostrar senha'}
          onClick={() => s.setShow(!s.show)}
        >
          {s.show ? 'Ocultar' : 'Mostrar'}
        </Button>
      </span>
    </label>
  );
}
function LoginControls({ s, p }: { s: Fields; p: AccessProps }) {
  return (
    <>
      <PasswordField s={s} />
      <div className="lg-row">
        <label className="lg-check">
          <input
            type="checkbox"
            checked={s.remember}
            onChange={(e) => s.setRemember(e.target.checked)}
          />{' '}
          Manter sessão
        </label>
        <Button
          className="link-control"
          label="Recuperar acesso"
          onClick={(e) => {
            e.preventDefault();
            s.change('recover');
          }}
        >
          Esqueci a senha
        </Button>
      </div>
      <Button
        className="lg-btn"
        label={s.mode === 'recover' ? 'Redefinir senha' : undefined}
        type="submit"
        disabled={p.busy || !p.connected}
      >
        {p.busy ? 'Aguarde…' : 'Entrar'}
      </Button>
      <p className="lg-alt">
        <Button
          className="link-control"
          disabled={!p.setup}
          onClick={() => s.change('register')}
        >
          Criar conta
        </Button>
      </p>
    </>
  );
}
export function AccessForm({ s, p }: { s: Fields; p: AccessProps }) {
  return (
    <form onSubmit={s.send}>
      <Field
        label={s.mode === 'reset' ? 'Novo usuário' : 'Usuário'}
        autoComplete="username"
        required
        maxLength={80}
        value={s.name}
        onChange={(e) => s.setName(e.target.value)}
      />
      {s.mode === 'login' ? (
        <LoginControls s={s} p={p} />
      ) : (
        <>
          {s.mode === 'recover' && (
            <Field
              label="Código de recuperação"
              required
              autoComplete="off"
              value={s.code}
              onChange={(e) => s.setCode(e.target.value)}
            />
          )}
          <Field
            label={
              s.mode === 'recover' || s.mode === 'reset'
                ? 'Nova senha'
                : 'Senha'
            }
            type="password"
            required
            maxLength={128}
            autoComplete="new-password"
            value={s.password}
            onChange={(e) => s.setPassword(e.target.value)}
          />
          {(s.mode === 'register' || s.mode === 'reset') && (
            <Field
              label={
                s.mode === 'reset' ? 'Confirmar nova senha' : 'Confirmar senha'
              }
              type="password"
              required
              maxLength={128}
              autoComplete="new-password"
              value={s.confirm}
              onChange={(e) => s.setConfirm(e.target.value)}
            />
          )}
          {s.mode === 'reset' && (
            <p className="muted">
              O sistema vai pedir a senha de administrador deste computador.
            </p>
          )}
          <Button
            className="lg-btn"
            type="submit"
            disabled={p.busy || !p.connected}
          >
            {p.busy
              ? 'Aguarde…'
              : s.mode === 'register'
                ? 'Criar conta local'
                : s.mode === 'reset'
                  ? 'Redefinir conta'
                  : 'Recuperar'}
          </Button>
          {s.mode === 'recover' && (
            <p className="lg-alt">
              <Button
                className="link-control"
                disabled={p.busy || p.resetVerification}
                title={
                  p.resetVerification
                    ? 'Tente entrar antes de solicitar outra redefinição.'
                    : undefined
                }
                onClick={() => s.change('reset')}
              >
                Perdi a senha e o código
              </Button>
            </p>
          )}
          <p className="lg-alt">
            <Button
              className="link-control"
              disabled={p.busy}
              onClick={(e) => {
                e.preventDefault();
                s.change('login');
              }}
            >
              Voltar ao login
            </Button>
          </p>
        </>
      )}
    </form>
  );
}
