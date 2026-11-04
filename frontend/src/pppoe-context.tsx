import {
  ContextCountsPolicyInputs,
  ContextGroupsList,
} from './pppoe-context-table';
import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';
import { Button } from './ui/Button';
import { useEffect, useState } from 'react';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import { pendingPPPoE as pending } from './equipment-receipts';
import {
  exactRate,
  formatDate,
  formatDuration,
  formatRate,
} from './network-format';
import { pppoeAPI as api } from './presentation';
import type {
  PPDelta,
  PPGroup,
  PPObservation,
  PPQuery,
  PPReference,
  PPResult,
  PPRow,
} from './presentation';
export const freshQuery = (): PPQuery => ({
  search: '',
  duplicates_only: false,
  sort: 'login',
  desc: false,
  category: '',
});
export const FRESH_QUERY = freshQuery;
export const pppoeMessage = (r: PPResult) =>
  r.error?.reason === 'operation_receipts'
    ? 'Limite de 2048 alterações permanentes atingido. Salvar e excluir referências estão bloqueados. ' +
      'Atualizar leituras, consultar, comparar e exportar continuam disponíveis.'
    : r.error?.reason?.startsWith('references')
      ? 'Limite de referências atingido. Exclua uma referência que não precisa mais.'
      : {
          OBSERVATION_UNAVAILABLE:
            'A leitura exibida não está mais disponível. Abra a leitura atual.',
          OBSERVATION_INCOMPATIBLE:
            'As leituras pertencem a contextos incompatíveis.',
          CURSOR_INVALID: 'A página expirou. Volte ao início da lista.',
        }[r.code] || equipmentMessage(r.code);
export const pppoeErrorMessage = pppoeMessage;
export const COMPARE_CATEGORIES: Record<string, string> = {
  unchanged: 'Sem alteração',
  changed: 'Campos alterados',
  ambiguous: 'Ambígua / possível reconexão',
  new: 'Somente na atual',
  absent: 'Somente na referência',
};
const SECRET_STATE_LABELS: Record<string, string> = {
  active: 'Um secret local ativo',
  disabled: 'Secret local desativado: a autenticação não vem dele',
  ambiguous: 'Vários secrets locais ativos: perfil ambíguo',
  none: 'Nenhum secret local para este login',
};
export function OfflineEvidence({ row }: { row: PPRow }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30000);
    return () => clearInterval(timer);
  }, []);
  const last = row.absent_end ? Date.parse(row.absent_end) : NaN;
  return (
    <dl className="pppoe-diagnostic-values pppoe-context-values">
      <dt>Offline desde (intervalo)</dt>
      <dd
        title={`${row.absent_start || 'Indisponível'} → ${row.absent_end || 'Indisponível'}`}
      >
        {formatDate(row.absent_start, true)} →{' '}
        {formatDate(row.absent_end, true)}. Última leitura ONLINE → primeira
        leitura em que faltou.
      </dd>
      <dt>Há quanto tempo (pelo menos)</dt>
      <dd>
        {formatDuration((now - last) / 1000)}. Limite inferior calculado da
        primeira ausência, sem estimar o instante da queda.
      </dd>
    </dl>
  );
}
export function SessionContext({
  row,
  omitted,
  fastTrack,
}: {
  row: PPRow;
  omitted?: boolean;
  fastTrack?: number;
}) {
  const c = row.context;
  if (!c && omitted)
    return (
      <p>
        Contexto não incluído por limite. VLAN, IPv6 e plano indisponíveis nesta
        leitura.
      </p>
    );
  if (!c || c.association !== 'associated')
    return (
      <p>Túnel: não associado. Dados de VLAN, IPv6 e plano indisponíveis.</p>
    );
  const planTitle = `${exactRate(c.download_limit)} download / ${exactRate(c.upload_limit)} upload`;
  const planText =
    c.upload_limit !== undefined && c.download_limit !== undefined
      ? `${formatRate(c.download_limit)} / ${formatRate(c.upload_limit)}`
      : 'Indisponível';
  return (
    <dl className="pppoe-diagnostic-values pppoe-context-values">
      <dt>Interface de origem</dt>
      <dd>{c.interface || 'Indisponível'}</dd>
      <dt>VLAN</dt>
      <dd>{c.vlan ?? 'Indisponível'}</dd>
      <dt>Serviço</dt>
      <dd>{c.service || 'Indisponível'}</dd>
      <dt>IPv6 negociado</dt>
      <dd>{c.ipv6 === undefined ? 'Indisponível' : c.ipv6 ? 'Sim' : 'Não'}</dd>
      <dt>Prefixo delegado</dt>
      <dd>
        {c.delegated_prefix ||
          (c.pd_status === 'none' ? 'Sem prefixo delegado' : 'Indisponível')}
      </dd>
      <dt>Secret local</dt>
      <dd>
        {SECRET_STATE_LABELS[c.secret_state || ''] ||
          'Não lido ou falha na leitura do secret local'}
      </dd>
      {c.secret_state === 'active' && (
        <>
          <dt>Perfil do secret ativo</dt>
          <dd>{c.profile || 'Indisponível'}</dd>
        </>
      )}
      <dt>Limite de plano (fila dinâmica) — download / upload</dt>
      <dd title={planTitle}>
        {planText}
        {fastTrack !== undefined && fastTrack > 0 && (
          <small className="cell-sub">limite lido, não garantido</small>
        )}
      </dd>
    </dl>
  );
}
function useContextGroupsFetch(
  profile: string,
  activation: string,
  device: string,
  observation: string,
  cursor: string,
) {
  const [groups, setGroups] = useState<PPGroup[]>([]),
    [next, setNext] = useState(''),
    [error, setError] = useState(''),
    [resolvedKey, setResolvedKey] = useState('');
  const key = [profile, activation, device, observation, cursor].join('|');
  useEffect(() => {
    let current = true;
    if (typeof api.groups !== 'function') return;
    void api
      .groups(profile, activation, device, observation, cursor)
      .then((r) => {
        if (!current) return;
        setResolvedKey(key);
        if (
          r.profile_id !== profile ||
          r.activation_id !== activation ||
          r.code !== 'OK'
        ) {
          setError('Contagem por VLAN e serviço indisponível.');
          return;
        }
        setGroups(r.data?.groups || []);
        setNext(r.data?.next_cursor || '');
        setError('');
      })
      .catch(() => {
        if (current) {
          setResolvedKey(key);
          setError('Contagem por VLAN e serviço indisponível.');
        }
      });
    return () => {
      current = false;
    };
  }, [profile, activation, device, observation, cursor, key]);
  return { groups, next, error, key, resolvedKey };
}
export function ContextCounts({
  profile,
  activation,
  device,
  observation,
  at,
}: {
  profile: string;
  activation: string;
  device: string;
  observation: string;
  at?: string;
}) {
  const [cursor, setCursor] = useState('');
  const [dropThreshold, setDropThreshold] = useState('50'),
    [minimumPeak, setMinimumPeak] = useState('5');
  const threshold = Number(dropThreshold),
    minimum = Number(minimumPeak);
  const validPolicy =
    dropThreshold.trim() !== '' &&
    minimumPeak.trim() !== '' &&
    Number.isFinite(threshold) &&
    threshold > 0 &&
    threshold <= 100 &&
    Number.isInteger(minimum) &&
    minimum >= 1 &&
    minimum <= 2048;
  const { groups, next, error, key, resolvedKey } = useContextGroupsFetch(
    profile,
    activation,
    device,
    observation,
    cursor,
  );
  return (
    <DataBlock
      title="Sessões por VLAN e serviço"
      source="SSH"
      age={readingAge(at)}
      data={{
        state: 'ready',
        content: (
          <>
            <div className="bar">
              <ContextCountsPolicyInputs
                dropThreshold={dropThreshold}
                setDropThreshold={setDropThreshold}
                minimumPeak={minimumPeak}
                setMinimumPeak={setMinimumPeak}
                validPolicy={validPolicy}
              />
            </div>
            {resolvedKey !== key ? (
              <p role="status">Lendo contagens salvas…</p>
            ) : error ? (
              <p role="status">{error}</p>
            ) : (
              <ContextGroupsList
                groups={groups}
                validPolicy={validPolicy}
                minimum={minimum}
                threshold={threshold}
              />
            )}
            {resolvedKey === key && next && (
              <Button type="submit" onClick={() => setCursor(next)}>
                Próximos grupos
              </Button>
            )}
            {cursor && (
              <Button type="submit" onClick={() => setCursor('')}>
                Primeiros grupos
              </Button>
            )}
            <p className="muted">
              Pico entre amostras salvas até esta leitura.
            </p>
          </>
        ),
      }}
    />
  );
}
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
