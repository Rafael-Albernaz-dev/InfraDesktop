import { AccessForm } from './access-fields';
import { AccessProps, useAccessFields } from './use-access-fields';
import { RecoveryActions } from './recovery-actions';
import { Button } from '../../shared/ui/Button';
import { ProductBrand } from '../../shared/ui/ProductBrand';
import { RecoveryCode } from '../../shared/ui/RecoveryCode';

export function AccessScreen(p: AccessProps) {
  const s = useAccessFields(p);
  const title = p.recovery
    ? 'Guarde seu código'
    : s.mode === 'register'
      ? 'Criar conta'
      : s.mode === 'recover'
        ? 'Recuperar acesso'
        : s.mode === 'reset'
          ? 'Redefinir conta'
          : 'Entrar';
  const subtitle = p.recovery
    ? 'Ele aparece só agora.'
    : s.mode === 'register'
      ? 'A conta fica só neste computador. Senha com no mínimo 8 caracteres.'
      : s.mode === 'recover'
        ? 'Use seu código e escolha uma senha nova, com no mínimo 8 caracteres.'
        : s.mode === 'reset'
          ? 'Empresas, equipamentos e credenciais continuam aqui.'
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
