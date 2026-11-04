import { useCredentialUsage } from './credential-usage';
import { useState } from 'react';
import type { useEquipmentState } from './equipment-state';
import { DefaultCredentialDetails } from './equipment-credential-form';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';

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
  const usage = useCredentialUsage(profile, activation);
  const rows = s.credentials.filter((c) =>
    c.alias.toLocaleLowerCase().includes(search.toLocaleLowerCase()),
  );
  return (
    <>
      <h1>Credenciais</h1>
      <div className="toolbar">
        <input
          aria-label="Buscar credencial"
          placeholder="Buscar"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
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
            const count = usage?.[credential.id] ?? (usage ? 0 : undefined);
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
                  {count === undefined
                    ? 'Não informado nesta leitura'
                    : `${count} equipamento${count === 1 ? '' : 's'}`}
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
