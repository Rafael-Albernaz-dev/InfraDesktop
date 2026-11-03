import { Button } from './desktop';
import type { Credential, Device } from './equipment-api';
import { latestSavedReading } from './equipment-reading';
import {
  formatDate,
  formatDuration,
  formatEquipmentRole,
  formatMetric,
} from './network-format';

function EquipmentInventoryRow(p: {
  d: Device;
  selected?: Device;
  displayNow: number;
  openDevice: (d: Device) => void;
}) {
  const { d, selected, displayNow, openDevice } = p;
  const saved = latestSavedReading(d);
  return (
    <tr
      className={'selectable ' + (selected?.id === d.id ? 'selected' : '')}
      tabIndex={0}
      aria-selected={selected?.id === d.id}
      onClick={() => openDevice(d)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          openDevice(d);
        }
      }}
    >
      <td>
        <span className="cell-name">{d.name}</span>
      </td>
      <td className="mono">{d.address}</td>
      <td className="os-cell">{saved.version || 'Desconhecido'}</td>
      <td>{d.role ? formatEquipmentRole(d.role) : 'Não definido'}</td>
      <td title={saved.exactUptime}>{saved.uptime}</td>
      <td>{formatMetric(saved.cpu, '%')}</td>
      <td>
        <span
          className={
            'text-state ' + (d.result || d.monitoring ? 'good' : 'muted')
          }
        >
          {d.active
            ? d.result || d.monitoring
              ? 'Leitura disponível'
              : 'Não consultado'
            : 'Inativo'}
        </span>
      </td>
      <td className="reading-time" title={saved.at}>
        {saved.at ? formatDate(saved.at) : 'Não consultado'}
        {saved.at && (
          <small className="cell-sub">
            Últimos dados conhecidos · há{' '}
            {formatDuration((displayNow - Date.parse(saved.at)) / 1000)}
          </small>
        )}
      </td>
    </tr>
  );
}

function EquipmentInventoryTable(p: {
  devices: Device[];
  selected?: Device;
  displayNow: number;
  openDevice: (d: Device) => void;
}) {
  return (
    <div
      className="table-scroll"
      tabIndex={0}
      aria-label="Lista de equipamentos"
    >
      <table className="app-table inventory-table">
        <thead>
          <tr>
            <th>Equipamento</th>
            <th>Endereço</th>
            <th className="os-cell">Sistema</th>
            <th>Papel</th>
            <th>Uptime</th>
            <th>CPU</th>
            <th>Consulta</th>
            <th>Última leitura</th>
          </tr>
        </thead>
        <tbody>
          {p.devices.map((d) => (
            <EquipmentInventoryRow key={d.id} {...p} d={d} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EquipmentInventoryEmpty(p: {
  search: string;
  inactive: boolean;
  total: number;
  locked: boolean;
  setSearch: (v: string) => void;
  setInactive: (v: boolean) => void;
  setCursor: (v: number) => void;
  setForm: (v: Device | 'new' | undefined) => void;
}) {
  return (
    <div className="empty-state">
      <h2>
        {p.search
          ? 'Nenhum resultado'
          : p.inactive
            ? 'Nenhum equipamento inativo'
            : p.total
              ? 'Nenhum equipamento ativo'
              : 'Nenhum equipamento cadastrado'}
      </h2>
      {p.search ? (
        <Button
          onClick={() => {
            p.setSearch('');
            p.setCursor(0);
          }}
        >
          Limpar busca
        </Button>
      ) : p.total > 0 && !p.inactive ? (
        <Button
          onClick={() => {
            p.setInactive(true);
            p.setCursor(0);
          }}
        >
          Mostrar inativos
        </Button>
      ) : !p.total && !p.inactive ? (
        <Button
          kind="primary"
          onClick={() => p.setForm('new')}
          disabled={p.locked}
        >
          Adicionar equipamento
        </Button>
      ) : null}
    </div>
  );
}

function EquipmentInventoryToolbar(p: {
  primaryInHeader: boolean;
  locked: boolean;
  search: string;
  inactive: boolean;
  connected: boolean;
  vault: boolean;
  setForm: (v: Device | 'new' | undefined) => void;
  setSearch: (v: string) => void;
  setInactive: (v: boolean) => void;
  setCursor: (v: number) => void;
  setError: (v: string) => void;
  setCredentialForm: (v: Credential | 'new' | undefined) => void;
}) {
  return (
    <div className="inventory-tools">
      {!p.primaryInHeader && (
        <Button
          kind="primary"
          onClick={() => p.setForm('new')}
          disabled={p.locked}
        >
          Adicionar equipamento
        </Button>
      )}
      <input
        aria-label="Buscar equipamento"
        placeholder="Buscar equipamento"
        value={p.search}
        onChange={(e) => {
          p.setSearch(e.target.value);
          p.setCursor(0);
        }}
      />
      <select
        aria-label="Estado dos equipamentos"
        value={p.inactive ? 'inactive' : 'active'}
        onChange={(e) => {
          p.setInactive(e.target.value === 'inactive');
          p.setCursor(0);
        }}
      >
        <option value="active">Ativos</option>
        <option value="inactive">Inativos</option>
      </select>
      <Button
        onClick={() => {
          p.setError('');
          p.setCredentialForm('new');
        }}
        disabled={!p.connected || !p.vault}
      >
        Credenciais
      </Button>
    </div>
  );
}

export function EquipmentInventoryPane(p: {
  detailOpen: boolean;
  total: number;
  search: string;
  inactive: boolean;
  connected: boolean;
  vault: boolean;
  primaryInHeader: boolean;
  locked: boolean;
  devices: Device[];
  selected?: Device;
  displayNow: number;
  ready: boolean;
  listError: string;
  cursor: number;
  next: number;
  setError: (v: string) => void;
  setCredentialForm: (v: Credential | 'new' | undefined) => void;
  setForm: (v: Device | 'new' | undefined) => void;
  setSearch: (v: string) => void;
  setInactive: (v: boolean) => void;
  setCursor: (v: number) => void;
  openDevice: (d: Device) => void;
}) {
  return (
    <section className="pane-main" hidden={p.detailOpen}>
      {p.ready && p.total === 0 && !p.search && !p.inactive && (
        <Button
          onClick={() => {
            p.setError('');
            p.setCredentialForm('new');
          }}
          disabled={!p.connected || !p.vault}
        >
          Credenciais
        </Button>
      )}
      {(p.total > 0 || p.search || p.inactive) && (
        <EquipmentInventoryToolbar {...p} />
      )}
      {p.devices.length > 0 && <EquipmentInventoryTable {...p} />}
      {!p.devices.length && p.ready && !p.listError && (
        <EquipmentInventoryEmpty {...p} />
      )}
      {(p.cursor > 0 || p.next > 0) && (
        <div className="list-footer">
          <span>{p.devices.length} equipamentos nesta página</span>
          <div>
            <Button disabled={p.cursor === 0} onClick={() => p.setCursor(0)}>
              Início
            </Button>
            <Button disabled={!p.next} onClick={() => p.setCursor(p.next)}>
              Próxima
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
