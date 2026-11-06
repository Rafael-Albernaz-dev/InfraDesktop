import { useCredentialUsage } from './credential-usage';
import { useState } from 'react';
import type { useEquipmentState } from './equipment-state';
import { DefaultCredentialDetails } from './equipment-credential-form';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';
import { Select } from './ui/Select';
import { RadioRoles } from './radio-contract';

export function EquipmentCredentials({
  s,
  profile,
  activation,
  locked,
}: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  locked: boolean;
}) {
  const [search, setSearch] = useState('');
  const [role, setRole] = useState('');
  const [use, setUse] = useState('');
  const usage = useCredentialUsage(profile, activation);
  const needle = search.toLocaleLowerCase();
  const rows = s.credentials.filter((c) =>
    `${c.alias} ${c.user}`.toLocaleLowerCase().includes(needle) &&
    (!role || s.credentialDefaults[role] === c.id) &&
    (!use || (use === 'used' ? c.links > 0 : c.links === 0)),
  );
  return (
    <>
      <h1>Credenciais</h1>
      <div className="toolbar">
        <input
          aria-label="Buscar credencial"
          placeholder="Buscar nome ou usuário"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Select aria-label="Padrão em" value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="">Padrão em: todos</option>
          <option value={RadioRoles.ap}>AP</option><option value={RadioRoles.cpe}>CPE</option>
          <option value={RadioRoles.ptp}>PTP</option>
        </Select>
        <Select aria-label="Uso" value={use} onChange={(e) => setUse(e.target.value)}>
          <option value="">Uso: todas</option><option value="used">Em uso</option><option value="unused">Sem uso</option>
        </Select>
        {(search || role || use) && <Button onClick={() => { setSearch(''); setRole(''); setUse(''); }}>Limpar filtros</Button>}
        <span className="grow" />
        <Button
          kind="primary"
          disabled={locked || !s.vault}
          onClick={() => s.setCredentialForm('new')}
        >
          Adicionar credencial
        </Button>
      </div>
      <DataTable>
        <thead>
          <tr>
            <th>Nome</th>
            <th>Usuário</th>
            <th>Usada por</th>
            <th>Padrão em</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((credential) => {
            const names = usage?.[credential.id]?.join(', ') || '—';
            const defaults = Object.entries(s.credentialDefaults)
              .filter(([, value]) => value === credential.id)
              .map(([role]) => role.toUpperCase())
              .join(', ');
            return (
              <tr
                key={credential.id}
                className="click"
                tabIndex={0}
                onClick={() => s.setCredentialForm(credential)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    s.setCredentialForm(credential);
                  }
                }}
              >
                <td>{credential.alias}</td>
                <td>{credential.user}</td>
                <td>
                  {usage ? (
                    <span className="credential-usage" title={names}>
                      {names}
                    </span>
                  ) : (
                    'Não informado nesta leitura'
                  )}
                </td>
                <td>{defaults || '—'}</td>
              </tr>
            );
          })}
          {!rows.length && (
            <tr>
              <td colSpan={4}>Nenhuma credencial encontrada</td>
            </tr>
          )}
        </tbody>
      </DataTable>
      <div className="sp10" />
      <div className="settings-card">
        <DefaultCredentialDetails
          {...s}
          profile={profile}
          activation={activation}
          locked={locked}
        />
      </div>
    </>
  );
}
