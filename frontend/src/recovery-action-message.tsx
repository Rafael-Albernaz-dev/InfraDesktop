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
