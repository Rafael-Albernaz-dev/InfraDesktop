import { accessErrorKind } from './access-error-state';
import { resetVerificationMessage } from './reset-verification-message';
export type Intent =
  | 'login'
  | 'register'
  | 'recover'
  | 'reset'
  | 'renew'
  | 'password'
  | 'company'
  | 'brand'
  | 'theme'
  | 'ack'
  | 'general';
// Intent → message → available action. Raw backend text is never displayed.
export function message(code: string, intent: Intent, reason?: string): string {
  const resetNotice = resetVerificationMessage(code);
  if (resetNotice) return resetNotice;
  if (
    intent === 'login' &&
    ['INVALID_REQUEST', 'UNAUTHENTICATED'].includes(code)
  )
    return 'Usuário ou senha não conferem';
  if (code === 'UNAUTHENTICATED') {
    if (accessErrorKind(code, intent, reason) === 'incorrect-password')
      return 'SENHA INCORRETA';
    return intent === 'recover'
      ? 'Usuário ou código não conferem.'
      : 'Entre novamente para continuar.';
  }
  if (code === 'INVALID_REQUEST') {
    if (intent === 'register' || intent === 'recover' || intent === 'reset')
      return 'Use uma nova senha de 8 a 128 caracteres e confira o usuário.';
    if (intent === 'company')
      return 'Use um nome de empresa de 1 a 80 caracteres.';
    return 'Confira os campos e tente novamente.';
  }
  const messages: Record<string, string> = {
    ADMIN_AUTH_DENIED:
      'A autorização de administrador foi negada. Confira a senha de administrador e tente novamente.',
    ADMIN_AUTH_CANCELLED: 'A autorização foi cancelada.',
    ADMIN_AUTH_UNAVAILABLE:
      'Não foi possível abrir a autorização de administrador neste computador.',
    VERSION_INCOMPATIBLE: 'Atualize o InfraDesktop para abrir esta instalação.',
    TRY_LATER: 'Aguarde alguns instantes e tente novamente.',
    BUSY: 'Há uma operação em andamento. Aguarde e tente novamente.',
    ACCOUNT_EXISTS:
      'Já existe uma conta nesta instalação. Entre ou recupere o acesso.',
    PROFILE_CHANGED:
      'A empresa ou o acesso mudou. Confira a seleção e tente novamente.',
    SERVICE_UNAVAILABLE:
      'Não foi possível acessar o serviço local. Tente novamente.',
    DEADLINE_EXCEEDED: 'A resposta demorou. Tente novamente.',
    BRAND_NAME_INVALID:
      'Use até 48 caracteres no nome visual, sem espaços nas pontas.',
    BRAND_LOGO_INVALID:
      'Escolha PNG, JPEG ou WebP de até 2 MB e 4096 × 4096 pixels.',
    BRAND_ACCENT_INVALID: 'Escolha uma das cores disponíveis.',
  };
  return messages[code] || 'Não foi possível concluir. Tente novamente.';
}
export { recoveryActionMessage } from './recovery-action-message';
