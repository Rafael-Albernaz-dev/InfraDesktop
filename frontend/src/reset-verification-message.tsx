export function resetVerificationMessage(code: string): string {
  if (
    code !== 'RESET_OUTCOME_UNKNOWN' &&
    code !== 'RESET_VERIFICATION_REQUIRED'
  )
    return '';
  return (
    'Não foi possível confirmar a redefinição. Tente entrar com o novo usuário e a nova senha antes de tentar novamente. ' +
    'Se entrar, gere um novo código em Configurações.'
  );
}
