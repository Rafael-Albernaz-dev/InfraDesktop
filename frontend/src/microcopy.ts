export type Intent =
  | 'login'
  | 'register'
  | 'recover'
  | 'renew'
  | 'company'
  | 'brand'
  | 'theme'
  | 'ack'
  | 'general';
// Intent → message → available action. Raw backend text is never displayed.
export function message(code: string, intent: Intent): string {
  if (
    intent === 'login' &&
    ['INVALID_REQUEST', 'UNAUTHENTICATED'].includes(code)
  )
    return 'Usuário ou senha não conferem';
  if (code === 'UNAUTHENTICATED')
    return intent === 'renew'
      ? 'A senha não confere. Tente novamente.'
      : intent === 'recover'
        ? 'Usuário ou código não conferem.'
        : 'Entre novamente para continuar.';
  if (code === 'INVALID_REQUEST') {
    if (intent === 'register' || intent === 'recover')
      return 'Use uma nova senha de 15 a 128 caracteres e confira o usuário.';
    if (intent === 'renew') return 'Confira sua senha atual.';
    if (intent === 'company')
      return 'Use um nome de empresa de 1 a 80 caracteres.';
    return 'Confira os campos e tente novamente.';
  }
  const messages: Record<string, string> = {
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
export function recoveryActionMessage(
  code: string,
  action: 'copy' | 'save',
): string {
  if (code === 'CANCELLED') return '';
  if (code === 'OK')
    return action === 'copy'
      ? 'Código copiado.'
      : 'Arquivo salvo. Confirme depois de guardá-lo.';
  if (code === 'SAVE_EXISTS')
    return 'Este arquivo já existe. Escolha outro nome.';
  if (code === 'SAVE_INVALID_DESTINATION')
    return 'Escolha um arquivo com extensão .txt.';
  if (code === 'PROFILE_CHANGED' || code === 'UNAUTHENTICATED')
    return 'Este código não está mais disponível. Recupere o acesso ou gere outro.';
  return action === 'copy'
    ? 'Não foi possível copiar. Selecione e copie o código manualmente.'
    : 'Não foi possível salvar. Tente outro destino ou copie o código.';
}
