import { DataTable } from './ui/DataTable';
import { useState } from 'react';
import type { ReactNode } from 'react';
import { Button, Logo } from './desktop-controls';
import type {
  Branding,
  Company,
  DesktopProps,
  Operations,
  Page,
} from './desktop-model';
import { exactNumber, formatAge } from './network-format';
function State({
  children,
  kind = '',
}: {
  children: ReactNode;
  kind?: string;
}) {
  return <span className={'text-state ' + kind}>{children}</span>;
}
function KV({
  label,
  children,
  mono = false,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="kv">
      <span className="key">{label}</span>
      <span className={mono ? 'mono' : undefined}>{children}</span>
    </div>
  );
}
function Empty({
  title = 'Nenhum equipamento',
  children,
}: {
  title?: string;
  children: ReactNode;
}) {
  return (
    <div className="empty-surface">
      <h1>{title}</h1>
      {children}
    </div>
  );
}
function CompanyRow({
  c,
  isSelected,
  onSelect,
}: {
  c: Company;
  isSelected: boolean;
  onSelect: (id: string) => void;
}) {
  return (
    <tr
      className={'company-row selectable ' + (isSelected ? 'selected' : '')}
      tabIndex={0}
      aria-selected={isSelected}
      aria-label={'Selecionar ' + c.name}
      onClick={() => onSelect(c.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(c.id);
        }
      }}
    >
      <td>
        <span className="cell-name">{c.name}</span>
        <span className="cell-sub">
          {c.lastAccess || 'Perfil desta instalação'}
        </span>
      </td>
      <td>
        <State kind={c.connectionStyle}>
          {c.connection || 'Ainda não consultada'}
        </State>
      </td>
      <td className="numeric">
        {c.count !== undefined ? exactNumber(c.count) : 'Ainda não consultado'}
      </td>
    </tr>
  );
}
function CompanyTable({
  companies,
  selectedId,
  onSelect,
}: {
  companies: Company[];
  selectedId?: string;
  onSelect: (id: string) => void;
}) {
  return (
    <DataTable className="data-table company-table">
      <thead>
        <tr>
          <th>Empresa</th>
          <th>Conexão</th>
          <th className="numeric">Equipamentos</th>
        </tr>
      </thead>
      <tbody>
        {companies.map((c) => (
          <CompanyRow
            key={c.id}
            c={c}
            isSelected={selectedId === c.id}
            onSelect={onSelect}
          />
        ))}
      </tbody>
    </DataTable>
  );
}
function CompanyDetail({
  selected,
  p,
}: {
  selected?: Company;
  p: DesktopProps;
}) {
  const brand =
    selected?.id === p.activeProfile
      ? p.brand
      : { name: selected?.name || '', logo: '', accent: 'teal' };
  return (
    <aside className="company-detail">
      <Logo brand={brand} size="large" />
      <h2>{selected?.name || 'Seu primeiro perfil'}</h2>
      <KV label="Rede">{selected?.network || 'Ainda não consultada'}</KV>
      <KV label="Inventário">
        {selected?.inventory || 'Inventário ainda não consultado'}
      </KV>
      <KV label="Acessos de equipamentos">
        {selected?.access || 'Acessos ainda não consultados'}
      </KV>
      <Button
        kind="primary"
        disabled={p.busy || !p.connected || !selected || selected.disabled}
        onClick={() => selected && p.enter(selected.id)}
      >
        Abrir empresa
      </Button>
    </aside>
  );
}
export function Companies(p: DesktopProps) {
  const [chosen, setChosen] = useState('');
  const selected =
    p.companies.find((c) => c.id === (chosen || p.activeProfile)) ||
    p.companies[0];
  const countLabel = `${p.companies.length} ${p.companies.length === 1 ? 'empresa' : 'empresas'} nesta instalação`;
  return (
    <>
      <div className="company-intro">
        <h1>Empresas</h1>
        <p>{countLabel}</p>
      </div>
      <div className="company-layout">
        <section className="company-list">
          {p.companies.length ? (
            <CompanyTable
              companies={p.companies}
              selectedId={selected?.id}
              onSelect={setChosen}
            />
          ) : (
            <Empty title="Nenhuma empresa cadastrada">
              <Button kind="primary" onClick={p.create} disabled={p.busy}>
                Criar empresa
              </Button>
            </Empty>
          )}
        </section>
        <CompanyDetail selected={selected} p={p} />
      </div>
    </>
  );
}
function DashboardInventoryTable(props: {
  ops: Operations;
  openEquipment?: (id: string) => void;
}) {
  const { ops, openEquipment } = props;
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Equipamentos da empresa"
    >
      <DataTable className="data-table">
        <thead>
          <tr>
            <th>Equipamento</th>
            <th>Sistema</th>
            <th>Estado da leitura</th>
            <th>Última leitura válida</th>
            <th>Sessões PPPoE</th>
          </tr>
        </thead>
        <tbody>
          {ops.devices.map((d) => (
            <tr key={d.id}>
              <td>
                <Button
                  kind="flat device-link"
                  onClick={() => openEquipment?.(d.id)}
                >
                  {d.name} · {d.ip}
                </Button>
              </td>
              <td>{d.os}</td>
              <td>{d.state}</td>
              <td>{d.time}</td>
              <td>
                {d.type === 'Concentrador'
                  ? d.pppoeAt
                    ? `${d.sessions ?? 0} ONLINE · ${d.offline ?? 0} OFFLINE · ` +
                      `${new Date(d.pppoeAt).toLocaleString('pt-BR')} (${formatAge(d.pppoeAt)})`
                    : 'Ainda não coletado'
                  : 'Não aplicável (não é concentrador)'}
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable>
    </div>
  );
}
function DashboardStatusView(props: {
  ops: Operations;
  brand: Branding;
  add: () => void;
  openEquipment?: (id: string) => void;
}) {
  const { ops, brand, add, openEquipment } = props;
  if (ops.inventoryState === 'loading') {
    return <p role="status">Carregando equipamentos…</p>;
  }
  if (ops.inventoryState === 'error') {
    return (
      <>
        <p role="alert">{ops.inventoryError}</p>
        <Button onClick={ops.retry}>Tentar novamente</Button>
      </>
    );
  }
  return (
    <>
      <p className="summary">
        {brand.name} · {ops.devices.length} equipamentos cadastrados · consultas
        sob demanda
      </p>
      {!ops.devices.length ? (
        <div className="empty-state">
          <h2>Nenhum equipamento cadastrado</h2>
          <Button kind="primary" onClick={add}>
            Adicionar equipamento
          </Button>
        </div>
      ) : (
        <DashboardInventoryTable ops={ops} openEquipment={openEquipment} />
      )}
      <p className="muted">
        Topologia, tráfego, alertas e histórico ainda não estão disponíveis. O
        serviço ativo indica apenas o serviço local.
      </p>
    </>
  );
}
export function Dashboard({
  ops,
  brand,
  navigate,
  add,
  inventoryAvailable,
  openEquipment,
}: {
  openEquipment?: (id: string) => void;
  inventoryAvailable?: boolean;
  ops: Operations;
  brand: Branding;
  navigate: (p: Page) => void;
  add: () => void;
}) {
  if (ops.inventoryState) {
    return (
      <section className="equipment-dashboard">
        <h1>Operação da rede</h1>
        <DashboardStatusView
          ops={ops}
          brand={brand}
          add={add}
          openEquipment={openEquipment}
        />
      </section>
    );
  }
  if (!ops.devices.length) {
    return (
      <Empty
        title={
          inventoryAvailable ? 'Inventário disponível nesta empresa' : undefined
        }
      >
        <Button kind="primary" onClick={add}>
          Adicionar equipamento
        </Button>
      </Empty>
    );
  }
  return (
    <section className="equipment-dashboard">
      <h1>Operação da rede</h1>
      <p className="summary">
        {brand.name} · {ops.devices.length} equipamentos cadastrados
      </p>
      <DashboardInventoryTable
        ops={ops}
        openEquipment={openEquipment || (() => navigate('equipment'))}
      />
    </section>
  );
}
export function PPPoERouteView(props: {
  ops: Operations;
  navigate: (p: Page) => void;
  openEquipment?: (id: string) => void;
}) {
  const { ops, navigate, openEquipment } = props;
  const concentrators = ops.devices.filter((d) => d.type === 'Concentrador');
  return (
    <div className="empty-state">
      <h2>Sessões PPPoE</h2>
      <p>Escolha um concentrador salvo e abra suas sessões no equipamento.</p>
      {concentrators.length > 0 && (
        <div className="action-cluster">
          {concentrators.map((d) => (
            <Button
              key={d.id}
              kind="flat"
              onClick={() =>
                openEquipment ? openEquipment(d.id) : navigate('equipment')
              }
            >
              {d.name} · {d.ip}
            </Button>
          ))}
        </div>
      )}
      <Button kind="primary" onClick={() => navigate('equipment')}>
        Ver equipamentos
      </Button>
    </div>
  );
}
export function TopologyRouteView(props: {
  ops: Operations;
  navigate: (p: Page) => void;
  openEquipment?: (id: string) => void;
}) {
  const { ops, navigate, openEquipment } = props;
  return (
    <div className="empty-state">
      <h2>Topologia ainda indisponível</h2>
      <p>
        Os equipamentos cadastrados estão no inventário da empresa. O mapa
        automático de enlaces físicos ainda não é coletado nesta versão.
      </p>
      {ops.devices.length > 0 && (
        <div className="action-cluster">
          {ops.devices.map((d) => (
            <Button
              key={d.id}
              kind="flat"
              onClick={() =>
                openEquipment ? openEquipment(d.id) : navigate('equipment')
              }
            >
              {d.name} · {d.ip}
            </Button>
          ))}
        </div>
      )}
      <Button kind="primary" onClick={() => navigate('equipment')}>
        Ver equipamentos
      </Button>
    </div>
  );
}
