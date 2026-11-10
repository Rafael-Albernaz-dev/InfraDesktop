import { equipmentMessage } from '../equipment/equipment-api';
const messages: Record<string, string> = {
  PING_CONTEXT_UNAVAILABLE:
    'O contexto main contém VRF, regras ou mangle ativos incompatíveis com a sondagem.',
  PING_SESSION_CHANGED:
    'A sessão mudou no concentrador. Atualize as sessões antes de sondar.',
  PING_READING_STALE:
    'A leitura pertence a um contexto anterior do serviço. Atualize as sessões.',
  PING_OFFLINE: 'Ping indisponível: sessão OFFLINE.',
  PARTIAL: 'O concentrador não devolveu uma leitura completa.',
  OBSERVATION_INCOMPATIBLE:
    'A leitura não permite uma sondagem segura. Confira a identidade da sessão.',
  OBSERVATION_UNAVAILABLE:
    'A leitura foi substituída. Reabra os dados armazenados.',
  JOB_INTERRUPTED:
    'A ação anterior foi interrompida. Ela não será reenviada automaticamente.',
};
export function diagnosticMessage(code: string) {
  return messages[code] || equipmentMessage(code);
}
