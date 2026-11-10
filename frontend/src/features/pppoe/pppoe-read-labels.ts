import type { ReadState, PPReadPanelProps } from './pppoe-read-state';
import { RadioModes } from '../radio/radio-contract';
export function readingFlag(v?: boolean) {
  if (v === undefined) return 'Não informado nesta leitura';
  return v ? 'Sim' : 'Não';
}
export function cancelReadLabel(s: ReadState, p: PPReadPanelProps) {
  if (p.traffic && (s.job || s.renewing)) return 'Parar';
  if (!s.job) return 'Cancelar solicitação pendente';
  if (s.scope === RadioModes.live && !s.deadlineReached)
    return 'Encerrar ao vivo';
  return 'Cancelar medição';
}
export function trafficReadNotice(s: ReadState) {
  if (s.result)
    return `Última atualização há ${Math.floor(s.sampleAge / 1000)} s.`;
  if (s.job || s.starting) return 'Aguardando primeira amostra.';
  return 'Nenhuma amostra recebida.';
}
