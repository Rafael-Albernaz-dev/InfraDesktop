import type { PPRow } from '../../platform/wails/presentation';
import { useEffect, useState } from 'react';
import { readingAge } from '../equipment/equipment-reading-format';
import {
  exactRate,
  formatDate,
  formatDuration,
  formatRate,
} from '../equipment/network-format';
import {
  ContextCountsPolicyInputs,
  ContextGroupsList,
} from './pppoe-context-table';
import type { PPGroup } from '../../platform/wails/pppoe-types';
import { pppoeAPI as api } from '../../platform/wails/presentation';
import { Button } from '../../shared/ui/Button';
import { DataBlock } from '../../shared/ui/DataBlock';
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
        title={`${row.absent_start || 'Não informado nesta leitura'} → ${row.absent_end || 'Não informado nesta leitura'}`}
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
      : 'Não informado nesta leitura';
  return (
    <dl className="pppoe-diagnostic-values pppoe-context-values">
      <dt>Interface de origem</dt>
      <dd>{c.interface || 'Não informado nesta leitura'}</dd>
      <dt>VLAN</dt>
      <dd>{c.vlan ?? 'Não informado nesta leitura'}</dd>
      <dt>Serviço</dt>
      <dd>{c.service || 'Não informado nesta leitura'}</dd>
      <dt>IPv6 negociado</dt>
      <dd>
        {c.ipv6 === undefined
          ? 'Não informado nesta leitura'
          : c.ipv6
            ? 'Sim'
            : 'Não'}
      </dd>
      <dt>Prefixo delegado</dt>
      <dd>
        {c.delegated_prefix ||
          (c.pd_status === 'none'
            ? 'Sem prefixo delegado'
            : 'Não informado nesta leitura')}
      </dd>
      <dt>Secret local</dt>
      <dd>
        {SECRET_STATE_LABELS[c.secret_state || ''] ||
          'Não lido ou falha na leitura do secret local'}
      </dd>
      {c.secret_state === 'active' && (
        <>
          <dt>Perfil do secret ativo</dt>
          <dd>{c.profile || 'Não informado nesta leitura'}</dd>
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
