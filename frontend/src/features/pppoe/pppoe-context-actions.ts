import type { Device } from '../equipment/equipment-api';
import { equipmentMessage } from '../equipment/equipment-api';
import { pendingPPPoE as pending } from '../equipment/equipment-receipts';
import type {
  PPResult,
  PPDelta,
  PPRow,
} from '../../platform/wails/presentation';
import type {
  PPObservation,
  PPQuery,
  PPReference,
} from '../../platform/wails/presentation-pppoe-types';
import { pppoeAPI as api } from '../../platform/wails/presentation';
import { RadioCodes } from '../radio/radio-contract';

export function ppReadMessage(code: string) {
  if (code === 'PARTIAL')
    return 'Não foi possível concluir esta leitura. Tente novamente.';
  return code === RadioCodes.UNSUPPORTED
    ? 'Esta leitura não está disponível neste formato, versão ou permissão.'
    : equipmentMessage(code);
}

export const freshQuery = (): PPQuery => ({
  search: '',
  duplicates_only: false,
  sort: 'login',
  desc: false,
  category: '',
});

export const FRESH_QUERY = freshQuery;

export const pppoeMessage = (r: PPResult) => {
  if (r.error?.reason === 'operation_receipts')
    return (
      'Limite de 2048 alterações permanentes atingido. Salvar e excluir referências estão bloqueados. ' +
      'Atualizar leituras, consultar, comparar e exportar continuam disponíveis.'
    );
  if (r.error?.reason?.startsWith('references'))
    return 'Limite de referências atingido. Exclua uma referência que não precisa mais.';
  return (
    {
      OBSERVATION_UNAVAILABLE:
        'A leitura exibida não está mais disponível. Abra a leitura atual.',
      OBSERVATION_INCOMPATIBLE:
        'As leituras pertencem a contextos incompatíveis.',
      CURSOR_INVALID: 'A página expirou. Volte ao início da lista.',
    }[r.code] || equipmentMessage(r.code)
  );
};

export const pppoeErrorMessage = pppoeMessage;

export const COMPARE_CATEGORIES: Record<string, string> = {
  unchanged: 'Sem alteração',
  changed: 'Campos alterados',
  ambiguous: 'Ambígua / possível reconexão',
  new: 'Somente na atual',
  absent: 'Somente na referência',
};

export type PPRunner = (
  action: () => Promise<PPResult>,
  done?: (r: PPResult) => void,
) => Promise<void>;

export type PPDetailRunner = (
  action: (accept: () => boolean) => Promise<PPResult>,
  done: (r: PPResult) => void,
) => Promise<void>;

export type PPHandlerProps = {
  profile: string;
  activation: string;
  device: Device;
  context: string;
  current?: PPObservation;
  base: string;
  setError: (s: string) => void;
  setNotice: (s: string) => void;
  setJob: (s: string) => void;
  setBase: React.Dispatch<React.SetStateAction<string>>;
  bump: React.Dispatch<React.SetStateAction<number>>;
  select: (r: PPRow | undefined) => void;
  setPair: (p: { left?: PPRow; right?: PPRow } | undefined) => void;
  removedFocus: React.MutableRefObject<string>;
  detailTrigger: React.MutableRefObject<HTMLElement | null>;
};

export function createUpdateHandler(p: PPHandlerProps, run: PPRunner) {
  return () => {
    const k = p.context + '|collect|' + p.device.revision;
    for (const [key, receipt] of pending)
      if (
        key.startsWith(p.context + '|collect|') &&
        receipt.revision !== undefined &&
        Number.isSafeInteger(receipt.revision) &&
        Number.isSafeInteger(p.device.revision) &&
        receipt.revision < p.device.revision
      )
        pending.delete(key);
    if (!pending.has(k) && pending.size >= 32) {
      p.setError(
        'Há 32 ações com resposta incerta. Reconcilie uma delas no contexto original.',
      );
      return;
    }
    const rec = pending.get(k) || {
      operation: crypto.randomUUID(),
      revision: p.device.revision,
    };
    pending.set(k, rec);
    void run(
      () =>
        api.start(
          p.profile,
          p.activation,
          p.device.id,
          rec.operation,
          p.device.revision,
        ),
      (r) => {
        if (r.data?.job) p.setJob(r.data.job.id);
      },
    );
  };
}

export function createSaveHandler(p: PPHandlerProps, run: PPRunner) {
  return () => {
    if (!p.current) return;
    const obsId = p.current.observation_id;
    const k = p.context + '|save|' + obsId;
    if (!pending.has(k) && pending.size >= 32) {
      p.setError('Limite de ações incertas; reconcilie no contexto original.');
      return;
    }
    const rec = pending.get(k) || {
      operation: crypto.randomUUID(),
      reference: crypto.randomUUID(),
    };
    pending.set(k, rec);
    void run(
      () =>
        api.save(
          p.profile,
          p.activation,
          p.device.id,
          obsId,
          rec.reference!,
          rec.operation,
        ),
      (r) => {
        pending.delete(k);
        p.setNotice(
          r.data?.effect_state === 'removed'
            ? 'Esta referência já foi removida. O replay não a recriou.'
            : 'Referência salva da leitura exibida.',
        );
        p.bump((n) => n + 1);
      },
    );
  };
}

export function createRemoveHandler(p: PPHandlerProps, run: PPRunner) {
  return (ref: PPReference) => {
    const k = p.context + '|delete|' + ref.reference_id;
    if (!pending.has(k) && pending.size >= 32) {
      p.setError('Limite de ações incertas; reconcilie no contexto original.');
      return;
    }
    const rec = pending.get(k) || { operation: crypto.randomUUID() };
    pending.set(k, rec);
    void run(
      () =>
        api.remove(
          p.profile,
          p.activation,
          p.device.id,
          ref.reference_id,
          rec.operation,
        ),
      () => {
        pending.delete(k);
        p.removedFocus.current = ref.reference_id;
        p.setBase((selectedBase) =>
          selectedBase === ref.observation_id ? '' : selectedBase,
        );
        p.bump((n) => n + 1);
      },
    );
  };
}

export function createOpenPairHandler(
  p: PPHandlerProps,
  runDetail: PPDetailRunner,
) {
  return async (d: PPDelta, trigger: HTMLElement) => {
    p.detailTrigger.current = trigger;
    let left: PPRow | undefined;
    let right: PPRow | undefined;
    await runDetail(
      async (accept) => {
        let r: PPResult = {
          code: 'OK',
          profile_id: p.profile,
          activation_id: p.activation,
        };
        if (d.base_row) {
          r = await api.detail(
            p.profile,
            p.activation,
            p.device.id,
            p.base,
            d.base_row,
          );
          if (!accept() || r.code !== 'OK') return r;
          left = r.data?.rows?.[0];
        }
        if (d.target_row) {
          r = await api.detail(
            p.profile,
            p.activation,
            p.device.id,
            p.current!.observation_id,
            d.target_row,
          );
          if (r.code !== 'OK') return r;
          right = r.data?.rows?.[0];
        }
        return r;
      },
      () => {
        p.select(undefined);
        p.setPair({ left, right });
      },
    );
  };
}
