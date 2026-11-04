import { DataBlock } from './ui/DataBlock';
import { DataTable } from './ui/DataTable';
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
      {(['log', 'secret'] as const).map((evidence) => (
        <PPPoEReadPanel
          key={evidence}
          profile={profile}
          activation={activation}
          device={device}
          observation={current.observation_id}
          rows={[]}
          logRow={selected.row_id}
          evidence={evidence}
          panel
          blocked={disabled || isOffline}
        />
      ))}
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
          Sessão {selected.login}
        </h2>
        <Button
          label="Fechar detalhe"
          onClick={() => {
            ++detailIntentRef.current;
            select(undefined);
            detailTriggerRef.current?.focus();
          }}
        >
          Fechar
        </Button>
      </div>
      <p className="muted">
        Concentrador: {device.name} · {selected.state || 'ONLINE'}
      </p>
      <SessionDetailNav detailTab={detailTab} setDetailTab={setDetailTab} />
      {detailTab === 'traffic' ? (
        selected.state === 'OFFLINE' ? (
          <DataBlock
            title="Tráfego"
            source={`SSH (${device.name})`}
            data={{
              state: 'ready',
              content: <p>Sem leitura: sessão OFFLINE</p>,
            }}
          />
        ) : (
          <>
            <PPPoEReadPanel
              profile={profile}
              activation={activation}
              device={device}
              observation={current.observation_id}
              rows={[selected.row_id]}
              traffic
              panel
              blocked={disabled}
            />
            <PPPoEReadPanel
              profile={profile}
              activation={activation}
              device={device}
              observation={current.observation_id}
              rows={[selected.row_id]}
              panel
              bandwidthOnly
              blocked={disabled}
            />
            <PPPoEReadPanel
              profile={profile}
              activation={activation}
              device={device}
              observation={current.observation_id}
              rows={[]}
              panel
              allowTop
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
      <div className="bar">
        <label>
          Referência para comparar
          <Select
            aria-label="Referência para comparar"
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
                Referência de {formatDate(ref.created_at)}
              </option>
            ))}
          </Select>
        </label>
        <label>
          Categoria
          <Select
            aria-label="Categoria"
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
      </div>
      <p className="muted">
        Ausência nesta leitura não comprova instante ou causa de queda. IDs e
        logins iguais podem ser ambíguos após reboot, reuso ou lacuna.
      </p>
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
  const reasonText =
    d.reason === 'reconnected' ? 'Reconectada' : COMPARE_CATEGORIES[d.category];
  return (
    <tr>
      <td>
        {d.target?.login || d.base?.login || 'Não informado nesta leitura'}
      </td>
      <td>{reasonText}</td>
      <td>
        {[
          d.address_changed ? 'IP alterado' : '',
          d.caller_changed ? 'Caller ID alterado' : '',
        ]
          .filter(Boolean)
          .join(' · ') || '—'}
      </td>
      <td>
        <Button
          type="submit"
          onClick={(e) => void openPair(d, e.currentTarget)}
        >
          Detalhar par
        </Button>
      </td>
    </tr>
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
      <DataTable>
        <thead>
          <tr>
            <th>Usuário</th>
            <th>Categoria</th>
            <th>Mudança</th>
            <th>Detalhe</th>
          </tr>
        </thead>
        <tbody>
          {p.deltas.map((d, i) => (
            <CompareDeltaItem
              key={d.base_row + '|' + d.target_row + '|' + i}
              d={d}
              openPair={p.openPair}
            />
          ))}
        </tbody>
      </DataTable>
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
    <tr>
      <td>Referência de {formatDate(refItem.created_at)}</td>
      <td>{refItem.count}</td>
      <td>
        <Button
          type="submit"
          disabled={disabled}
          aria-label={'Excluir referência ' + refItem.reference_id.slice(0, 8)}
          onClick={() => remove(refItem)}
        >
          Excluir referência
        </Button>
      </td>
    </tr>
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
    referencesHeadingRef,
    referencesState,
    setError,
    bump,
    savedReferences,
    remove,
  } = p;
  return (
    <>
      <h2 className="sr-only" tabIndex={-1} ref={referencesHeadingRef}>
        Referências preservadas
      </h2>
      <div className="bar">
        {current && (
          <span>
            Leitura atual: {formatDate(current.provenance.ended_at)} ·{' '}
            {current.count} registros
          </span>
        )}
        <span className="grow" />
        <Button
          disabled={disabled || referencesState === 'loading'}
          onClick={() => {
            setError('');
            bump((n) => n + 1);
          }}
        >
          Recarregar referências
        </Button>
      </div>
      {referencesState === 'loading' && (
        <p role="status">Carregando referências…</p>
      )}
      {referencesState === 'ready' && !savedReferences.length && (
        <p>Nenhuma referência salva.</p>
      )}
      {!!savedReferences.length && (
        <DataTable>
          <thead>
            <tr>
              <th>Referência</th>
              <th>Sessões</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {savedReferences.map((ref) => (
              <ReferenceItem
                key={ref.reference_id}
                refItem={ref}
                disabled={disabled}
                remove={remove}
              />
            ))}
          </tbody>
        </DataTable>
      )}
    </>
  );
}
