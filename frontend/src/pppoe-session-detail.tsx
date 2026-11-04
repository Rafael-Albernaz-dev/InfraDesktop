import { SessionAuthInfo, SessionConnectivityTab } from './pppoe-session-info';
import { Select } from './ui/Select';
import { Button } from './ui/Button';
import { Tabs } from './ui/Tabs';
import type { Device } from './equipment-api';
import { formatDate, uptimeTitle } from './network-format';
import { COMPARE_CATEGORIES } from './pppoe-context';
import { PPPoEReadPanel } from './pppoe-diagnostics';
import { formatPPUptime } from './pppoe-format';
import type {
  PPDelta,
  PPObservation,
  PPQuery,
  PPReference,
  PPRow,
} from './presentation';
import { RadioLinkPanel } from './radio';

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
  detailTab: 'traffic' | 'information' | 'link';
  setDetailTab: (t: 'traffic' | 'information' | 'link') => void;
}) {
  return (
    <Tabs
      label="Detalhes da sessão"
      items={[
        ['traffic', 'Tráfego'],
        ['information', 'Informações'],
        ['link', 'Enlace'],
      ]}
      value={detailTab}
      onChange={setDetailTab}
    />
  );
}
export function PPPoESessionDetail(p: {
  selected: PPRow;
  current: PPObservation;
  detailHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  detailTab: 'traffic' | 'information' | 'link';
  setDetailTab: (t: 'traffic' | 'information' | 'link') => void;
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
    <aside className="drawer session-panel" aria-label="Detalhes da sessão">
      <div className="dh">
        <h2 tabIndex={-1} ref={detailHeadingRef}>
          {selected.login}
        </h2>
        <Button
          label="Fechar detalhe"
          onClick={() => {
            ++detailIntentRef.current;
            select(undefined);
            detailTriggerRef.current?.focus();
          }}
        >
          ×
        </Button>
      </div>
      <SessionDetailNav detailTab={detailTab} setDetailTab={setDetailTab} />
      {detailTab === 'traffic' ? (
        selected.state === 'OFFLINE' ? (
          <p>Sem leitura: sessão OFFLINE</p>
        ) : (
          <>
            <PPPoEReadPanel
              profile={profile}
              activation={activation}
              device={device}
              observation={current.observation_id}
              rows={[selected.row_id]}
              traffic
              blocked={disabled}
            />
            <PPPoEReadPanel
              profile={profile}
              activation={activation}
              device={device}
              observation={current.observation_id}
              rows={[selected.row_id]}
              allowTop
              bandwidthOnly
              blocked={disabled}
            />
          </>
        )
      ) : detailTab === 'link' ? (
        <RadioLinkPanel
          profile={profile}
          activation={activation}
          device={device}
          observation={current.observation_id}
          row={selected.row_id}
          pppoeUptime={formatPPUptime(selected.uptime)}
          blocked={disabled || selected.state === 'OFFLINE'}
          onOpenEquipment={openEquipment}
        />
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
      <Button
        type="submit"
        onClick={() => {
          ++detailIntentRef.current;
          setPair(undefined);
          detailTriggerRef.current?.focus();
        }}
      >
        Fechar par
      </Button>
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
        <Select
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
        </Select>
      </label>
      <p>
        Ausência nesta leitura não comprova instante ou causa de queda. IDs e
        logins iguais podem ser ambíguos após reboot, reuso ou lacuna.
      </p>
      <label>
        Categoria
        <Select
          value={p.category}
          onChange={(e) => p.onChangeCategory(e.target.value)}
        >
          <option value="">Todas</option>
          {Object.entries(COMPARE_CATEGORIES).map(([id, label]) => (
            <option key={id} value={id}>
              {label}
            </option>
          ))}
        </Select>
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
      <Button type="submit" onClick={(e) => void openPair(d, e.currentTarget)}>
        Detalhar par
      </Button>
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
      <Button
        type="submit"
        disabled={disabled}
        aria-label={'Excluir referência ' + refItem.reference_id.slice(0, 8)}
        onClick={() => remove(refItem)}
      >
        Excluir referência
      </Button>
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
          <Button
            type="submit"
            className="primary"
            disabled={disabled}
            onClick={save}
          >
            Salvar referência desta leitura
          </Button>
        </>
      )}
      <h2 tabIndex={-1} ref={referencesHeadingRef}>
        Referências preservadas
      </h2>
      {referencesState === 'loading' && (
        <p role="status">Carregando referências…</p>
      )}
      {referencesState === 'error' && !initialFailed && (
        <Button
          type="submit"
          onClick={() => {
            setError('');
            bump((n) => n + 1);
          }}
        >
          Recarregar referências
        </Button>
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
