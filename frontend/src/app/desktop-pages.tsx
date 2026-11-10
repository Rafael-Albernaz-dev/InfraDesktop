import type { DesktopProps } from '../features/access/desktop-model';
import { SettingsPassword } from '../features/access/settings-password';
import { Button } from '../shared/ui/Button';
import { DataTable } from '../shared/ui/DataTable';
import { ProductBrand } from '../shared/ui/ProductBrand';
export function CompanyList({ p }: { p: DesktopProps }) {
  return (
    <div className="solo companies-page">
      <div className="lg-brand">
        <ProductBrand />
      </div>
      <div className="card">
        <div className="toolbar">
          <h1 className="grow">Empresas</h1>
          <Button
            kind="primary"
            disabled={p.busy || !p.connected}
            onClick={p.create}
          >
            Criar empresa
          </Button>
        </div>
        <DataTable>
          <thead>
            <tr>
              <th>Empresa</th>
              <th>Equipamentos</th>
            </tr>
          </thead>
          <tbody>
            {p.companies.map((company) => (
              <tr
                key={company.id}
                className="click"
                tabIndex={0}
                onClick={() => !company.disabled && p.enter(company.id)}
                onKeyDown={(e) => {
                  if (
                    (e.key === 'Enter' || e.key === ' ') &&
                    !company.disabled
                  ) {
                    e.preventDefault();
                    p.enter(company.id);
                  }
                }}
              >
                <td>
                  {company.name}
                  {company.id === p.activeProfile && <span> · Atual</span>}
                </td>
                <td>
                  {company.count === undefined
                    ? 'Não informado nesta leitura'
                    : company.count
                      ? `${company.count} equipamentos`
                      : 'Nenhum equipamento'}
                </td>
              </tr>
            ))}
            {!p.companies.length && (
              <tr>
                <td colSpan={2}>Nenhuma empresa cadastrada</td>
              </tr>
            )}
          </tbody>
        </DataTable>
      </div>
      {!p.companies.length && (
        <Button onClick={() => p.navigate('settings')}>Configurações</Button>
      )}
      {p.error && <p role="alert">{p.error}</p>}
    </div>
  );
}
export function Settings({ p }: { p: DesktopProps }) {
  return (
    <>
      <h1>Configurações</h1>
      <SettingsPassword p={p} />
      <div className="card settings-card">
        <h2>Código de recuperação</h2>
        <Button onClick={p.security} disabled={p.busy || !p.connected}>
          Gerar novo código
        </Button>
      </div>
    </>
  );
}
