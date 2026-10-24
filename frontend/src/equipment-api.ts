export { equipmentAPI } from './presentation';
export type {
  Device,
  Credential,
  Job,
  EquipmentResult,
  DeviceInput,
  CredentialInput,
} from './presentation';
export const equipmentMessage = (code: string) =>
  ({
    VAULT_UNAVAILABLE:
      'Cofre indisponível. Preserve os dados e confira a chave desta instalação.',
    REVISION_CONFLICT:
      'O cadastro mudou. Feche o formulário e confira a versão atual antes de gravar.',
    CREDENTIAL_NOT_FOUND: 'Selecione uma credencial de consulta desta empresa.',
    DEVICE_INACTIVE: 'Reative o equipamento antes de consultar.',
    TRUST_REQUIRED:
      'Confirme a primeira conexão para salvar a chave deste equipamento.',
    TRUST_CHANGED:
      'A chave do destino mudou. Consulta bloqueada; revise o destino e a identidade fora desta conexão.',
    REMOTE_AUTH_FAILED:
      'O destino recusou a autenticação. Confira o acesso sem repetir tentativas automaticamente.',
    REMOTE_PERMISSION_DENIED: 'O acesso não tem permissão para esta leitura.',
    PARTIAL:
      'A resposta está incompleta ou inválida. A última leitura válida desta parte foi preservada.',
    UNSUPPORTED: 'A resposta não identifica uma família RouterOS suportada.',
    CONNECT_FAILED:
      'Não foi possível alcançar a porta SSH. Confira destino, porta e a rede ou VPN disponível.',
    SSH_NEGOTIATION_FAILED:
      'A negociação SSH falhou. Confira a compatibilidade do destino; a segurança não foi reduzida.',
    SSH_CONNECTION_LOST:
      'A conexão SSH foi interrompida. A senha não foi diagnosticada como incorreta.',
    PERSISTENCE_FAILED:
      'A leitura não pôde ser salva. A última leitura válida foi preservada.',
    DEADLINE_EXCEEDED:
      'O prazo da operação terminou. Confira o estado antes de tentar novamente.',
    JOB_INTERRUPTED:
      'Esta consulta foi interrompida ou expirou. Inicie outra consulta deliberadamente.',
    CANCELLED:
      'Operação cancelada. A última leitura válida desta parte foi preservada.',
    BUSY: 'Duas operações já estão em curso. Aguarde ou cancele uma delas.',
    LIMIT_EXCEEDED: 'O limite desta operação foi atingido.',
    PROFILE_CHANGED: 'A empresa ou a sessão mudou. Abra o cadastro novamente.',
    UNAUTHENTICATED: 'Entre novamente para acessar os dados.',
    INVALID_REQUEST:
      'O serviço recusou os dados do cadastro. Confira os campos e abra novamente se o erro persistir.',
    SERVICE_UNAVAILABLE:
      'O serviço está indisponível. Tente novamente quando estiver ativo.',
  })[code] || 'Não foi possível concluir a operação.';
