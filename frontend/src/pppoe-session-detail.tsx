import type { Device } from './equipment-api';
import { formatDate, uptimeTitle } from './network-format';
import {
  COMPARE_CATEGORIES,
  OfflineEvidence,
  SessionContext,
} from './pppoe-context';
import { PPPoEPing, PPPoEReadPanel } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import type {
  PPDelta,
  PPInterval,
  PPObservation,
  PPQuery,
  PPReference,
  PPRow,
} from './presentation';
import { RadioLinkPanel } from './radio';

function SessionAuthInfo(p: {
  selected: PPRow;
  current: PPObservation;
  fastTrack?: number;
}) {
  const { selected, current } = p;
  return (
    <>
      <h3>Autenticação PPPoE</h3>
      <p>IP: {selected.address || 'Indisponível'}</p>
      <p>Caller ID: {selected.caller_id || 'Indisponível'}</p>
      <p title={uptimeTitle(selected.uptime)}>
        {selected.state === 'OFFLINE'
          ? 'Duração da última sessão: '
          : 'Tempo conectado: '}
        {formatPPUptime(selected.uptime)}
      </p>
      <p>Estado: {selected.state || 'ONLINE'}</p>
      <SessionContext
        row={selected}
        omitted={current.context_omitted}
        fastTrack={p.fastTrack}
      />
      <p>ID no roteador: {selected.remote_id || 'Indisponível'}</p>
      <p>Session ID: {selected.session_id || 'Indisponível'}</p>
      {selected.state === 'OFFLINE' && <OfflineEvidence row={selected} />}
    </>
  );
}

function SessionConnectivityTab(p: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  uptime: PPInterval;
  disabled: boolean;
  isOffline: boolean;
  openEquipment?: (id: string) => void;
}) {
  return (
    <>
      {!p.isOffline && (
        <RadioLinkPanel
          key={[p.profile, p.activation, p.observation, p.row].join('|')}
          profile={p.profile}
          activation={p.activation}
          device={p.device}
          observation={p.observation}
          row={p.row}
          pppoeUptime={formatPPUptime(p.uptime)}
          blocked={p.disabled}
          onOpenEquipment={p.openEquipment}
        />
      )}
      <h3>Conectividade</h3>
      <PPPoEPing
        key={p.observation + '|' + p.row}
        profile={p.profile}
        activation={p.activation}
        device={p.device}
        observation={p.observation}
        row={p.row}
        blocked={p.disabled || p.isOffline}
      />
      {p.isOffline && <p>Ping indisponível: sessão OFFLINE.</p>}
    </>
  );
}

function SessionInformationTab(p: {
  selected: PPRow;
  current: PPObservation;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  openEquipment?: (id: string) => void;
}) {
  const { selected, current, profile, activation, device, disabled } = p;
  const isOffline = selected.state === 'OFFLINE';
  return (
    <>
      <SessionAuthInfo
        selected={selected}
        current={current}
        fastTrack={p.fastTrack}
      />
      <PPPoEReadPanel
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        rows={isOffline ? [] : [selected.row_id]}
        blocked={disabled}
        allowTop
        logRow={selected.row_id}
      />
      <SessionConnectivityTab
        profile={profile}
        activation={activation}
        device={device}
        observation={current.observation_id}
        row={selected.row_id}
        uptime={selected.uptime}
        disabled={disabled}
        isOffline={isOffline}
        openEquipment={p.openEquipment}
      />
    </>
  );
}

function SessionDetailNav({
  detailTab,
  setDetailTab,
}: {
  detailTab: 'traffic' | 'information';
  setDetailTab: (t: 'traffic' | 'information') => void;
}) {
  return (
    <nav aria-label="Abas do detalhe PPPoE">
      <button
        aria-pressed={detailTab === 'traffic'}
        onClick={() => setDetailTab('traffic')}
      >
        Tráfego
      </button>
      <button
        aria-pressed={detailTab === 'information'}
        onClick={() => setDetailTab('information')}
      >
        Informações e ping
      </button>
    </nav>
  );
}

export function PPPoESessionDetail(p: {
  selected: PPRow;
  current: PPObservation;
  detailHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  detailTab: 'traffic' | 'information';
  setDetailTab: (t: 'traffic' | 'information') => void;
  profile: string;
  activation: string;
  device: Device;
  disabled: boolean;
  fastTrack?: number;
  detailIntentRef: React.MutableRefObject<number>;
  select: (r: PPRow | undefined) => void;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  openEquipment?: (id: string) => void;
}) {
  const {
    selected,
    current,
    detailHeadingRef,
    detailTab,
    setDetailTab,
    profile,
    activation,
    device,
    disabled,
    fastTrack,
    detailIntentRef,
    select,
    detailTriggerRef,
    openEquipment,
  } = p;
  return (
    <aside aria-label="Detalhes da sessão">
      <h2 tabIndex={-1} ref={detailHeadingRef}>
        {selected.login}
      </h2>
      <SessionDetailNav detailTab={detailTab} setDetailTab={setDetailTab} />
      {detailTab === 'traffic' ? (
        selected.state === 'OFFLINE' ? (
          <p>Tráfego indisponível: sessão OFFLINE.</p>
        ) : (
          <PPPoEReadPanel
            profile={profile}
            activation={activation}
            device={device}
            observation={current.observation_id}
            rows={[selected.row_id]}
            traffic
            blocked={disabled}
          />
        )
      ) : (
        <SessionInformationTab
          selected={selected}
          current={current}
          profile={profile}
          activation={activation}
          device={device}
          disabled={disabled}
          fastTrack={fastTrack}
          openEquipment={openEquipment}
        />
      )}
      <button
        onClick={() => {
          ++detailIntentRef.current;
          select(undefined);
          detailTriggerRef.current?.focus();
        }}
      >
        Fechar detalhe
      </button>
    </aside>
  );
}

export function PPPoEPairDetail(p: {
  pair: { left?: PPRow; right?: PPRow };
  pairHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  detailIntentRef: React.MutableRefObject<number>;
  setPair: (v: { left?: PPRow; right?: PPRow } | undefined) => void;
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
}) {
  const { pair, pairHeadingRef, detailIntentRef, setPair, detailTriggerRef } =
    p;
  return (
    <aside aria-label="Detalhes da comparação">
      <h2 tabIndex={-1} ref={pairHeadingRef}>
        Sessões do par
      </h2>
      {(['left', 'right'] as const).map((side) => (
        <div key={side}>
          <h3>{side === 'left' ? 'Referência' : 'Atual'}</h3>
          {pair[side] ? (
            <>
              <p>Login: {pair[side]!.login}</p>
              <p>IP: {pair[side]!.address || 'Indisponível'}</p>
              <p>Caller ID: {pair[side]!.caller_id || 'Indisponível'}</p>
              <p title={uptimeTitle(pair[side]!.uptime)}>
                Tempo conectado: {formatPPUptime(pair[side]!.uptime)}
              </p>
              <p>Estado: {pair[side]!.state || 'ONLINE'}</p>
            </>
          ) : (
            <p>Ausente neste conjunto.</p>
          )}
        </div>
      ))}
      <button
        onClick={() => {
          ++detailIntentRef.current;
          setPair(undefined);
          detailTriggerRef.current?.focus();
        }}
      >
        Fechar par
      </button>
    </aside>
  );
}

function formatSideSummary(row?: Partial<PPRow>, emptyLabel = 'Ausente') {
  if (!row) return emptyLabel;
  const uptimeText = row.uptime
    ? formatPPUptime(row.uptime)
    : 'Tempo indisponível';
  const prefix = `${row.login || 'Indisponível'} · ${row.address || 'IP indisponível'}`;
  return `${prefix} · ${row.caller_id || 'Caller ID indisponível'} · ${uptimeText} · ${row.state || 'ONLINE'}`;
}

function CompareControls(p: {
  base: string;
  setBase: (v: string) => void;
  setCursor: (v: string) => void;
  setHistory: (v: string[]) => void;
  refs: PPReference[];
  category: string;
  onChangeCategory: (c: string) => void;
}) {
  return (
    <>
      <label>
        Referência para comparar
        <select
          value={p.base}
          onChange={(e) => {
            p.setBase(e.target.value);
            p.setCursor('');
            p.setHistory([]);
          }}
        >
          <option value="">Selecione uma referência</option>
          {p.refs.map((ref) => (
            <option key={ref.reference_id} value={ref.observation_id}>
              {formatDate(ref.created_at)} · {ref.count} sessões
            </option>
          ))}
        </select>
      </label>
      <p>
        Ausência nesta leitura não comprova instante ou causa de queda. IDs e
        logins iguais podem ser ambíguos após reboot, reuso ou lacuna.
      </p>
      <label>
        Categoria
        <select
          value={p.category}
          onChange={(e) => p.onChangeCategory(e.target.value)}
        >
          <option value="">Todas</option>
          {Object.entries(COMPARE_CATEGORIES).map(([id, label]) => (
            <option key={id} value={id}>
              {label}
            </option>
          ))}
        </select>
      </label>
    </>
  );
}

function CompareDeltaItem({
  d,
  openPair,
}: {
  d: PPDelta;
  openPair: (d: PPDelta, trigger: HTMLElement) => Promise<void>;
}) {
  const baseTitle = d.base ? uptimeTitle(d.base.uptime) : 'Ausente';
  const targetTitle = d.target ? uptimeTitle(d.target.uptime) : 'Ausente';
  const reasonText =
    d.reason === 'reconnected' ? 'Reconectada' : COMPARE_CATEGORIES[d.category];
  return (
    <div>
      <span title={`${baseTitle} → ${targetTitle}`}>
        {formatSideSummary(d.base, 'Ausente na referência')} →{' '}
        {formatSideSummary(d.target, 'Ausente na atual')} · {reasonText}
      </span>
      {d.address_changed ? ' · IP alterado' : ''}
      {d.caller_changed ? ' · Caller ID alterado' : ''}
      <button onClick={(e) => void openPair(d, e.currentTarget)}>
        Detalhar par
      </button>
    </div>
  );
}

export function PPPoECompareView(p: {
  base: string;
  setBase: (v: string) => void;
  setCursor: (v: string) => void;
  setHistory: (v: string[]) => void;
  refs: PPReference[];
  query: PPQuery;
  change: (q: Partial<PPQuery>) => void;
  pageState: 'idle' | 'loading' | 'ready' | 'error';
  deltas: PPDelta[];
  openPair: (d: PPDelta, trigger: HTMLElement) => Promise<void>;
}) {
  return (
    <>
      <CompareControls
        base={p.base}
        setBase={p.setBase}
        setCursor={p.setCursor}
        setHistory={p.setHistory}
        refs={p.refs}
        category={p.query.category}
        onChangeCategory={(category) => p.change({ category })}
      />
      {p.pageState === 'loading' && <p role="status">Carregando comparação…</p>}
      {p.deltas.map((d, i) => (
        <CompareDeltaItem
          key={d.base_row + '|' + d.target_row + '|' + i}
          d={d}
          openPair={p.openPair}
        />
      ))}
    </>
  );
}

function ReferenceItem({
  refItem,
  disabled,
  remove,
}: {
  refItem: PPReference;
  disabled: boolean;
  remove: (ref: PPReference) => void;
}) {
  return (
    <div className="pppoe-reference">
      <span>
        {formatDate(refItem.created_at)} · {refItem.count} sessões
      </span>
      <button
        disabled={disabled}
        aria-label={'Excluir referência ' + refItem.reference_id.slice(0, 8)}
        onClick={() => remove(refItem)}
      >
        Excluir referência
      </button>
    </div>
  );
}

export function PPPoEReferencesView(p: {
  current?: PPObservation;
  disabled: boolean;
  save: () => void;
  referencesHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  referencesState: 'loading' | 'ready' | 'error';
  initialFailed: boolean;
  setError: (v: string) => void;
  bump: React.Dispatch<React.SetStateAction<number>>;
  savedReferences: PPReference[];
  remove: (ref: PPReference) => void;
}) {
  const {
    current,
    disabled,
    save,
    referencesHeadingRef,
    referencesState,
    initialFailed,
    setError,
    bump,
    savedReferences,
    remove,
  } = p;
  return (
    <>
      {current && (
        <>
          <p>
            Leitura atual: {formatDate(current.provenance.ended_at)} ·{' '}
            {current.count} registros
          </p>
          <button className="primary" disabled={disabled} onClick={save}>
            Salvar referência desta leitura
          </button>
        </>
      )}
      <h2 tabIndex={-1} ref={referencesHeadingRef}>
        Referências preservadas
      </h2>
      {referencesState === 'loading' && (
        <p role="status">Carregando referências…</p>
      )}
      {referencesState === 'error' && !initialFailed && (
        <button
          onClick={() => {
            setError('');
            bump((n) => n + 1);
          }}
        >
          Recarregar referências
        </button>
      )}
      {referencesState === 'ready' && !savedReferences.length && (
        <p>Nenhuma referência salva.</p>
      )}
      {savedReferences.map((ref) => (
        <ReferenceItem
          key={ref.reference_id}
          refItem={ref}
          disabled={disabled}
          remove={remove}
        />
      ))}
    </>
  );
}
