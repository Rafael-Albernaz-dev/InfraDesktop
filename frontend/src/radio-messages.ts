import { equipmentMessage } from './equipment-api';

export const radioMessage = (code: string) =>
  ({
    RADIO_SESSION_OFFLINE: 'Enlace indisponível: sessão OFFLINE.',
    RADIO_CPE_UNREACHABLE:
      'A CPE cadastrada não corresponde ao IP desta sessão. Confira o vínculo e o caminho de gestão.',
    RADIO_LINK_REQUIRED:
      'Confirme o vínculo com uma CPE e um AP cadastrados antes de consultar.',
    RADIO_LINK_UNCONFIRMED:
      'Vínculo não confirmado agora. Confira MAC, AP e interface; o vínculo salvo foi preservado.',
    RADIO_FAMILY_CONFIRMATION_REQUIRED:
      'Abra o equipamento, consulte e confirme a família detectada.',
    RADIO_LIVE_COST_REQUIRED: 'Teste a carga do ao vivo antes de iniciar.',
    RADIO_LIVE_ABORTED:
      'Ao vivo interrompido após duas falhas ou ultrapassagens consecutivas de duração, CPU ou ping.',
    RADIO_WINDOW_ENDED:
      'A janela do ao vivo terminou. Inicie outra observação deliberadamente.',
    RADIO_LOAD_UNAVAILABLE:
      'Medição de CPU e ping indisponível; ao vivo desligado.',
    OBSERVATION_UNAVAILABLE:
      'Os dados foram atualizados. Reabra os dados armazenados neste contexto.',
    UNSUPPORTED:
      'Este driver não identifica um rádio suportado neste firmware.',
  })[code] || equipmentMessage(code);
