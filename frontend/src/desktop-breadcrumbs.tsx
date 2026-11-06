import { useContext } from 'react';
import type { DesktopProps, RouteName } from './desktop-model';
import { SessionRoute } from './session-navigation';
type BreadcrumbProps = {
  p: DesktopProps;
  route: RouteName;
  detailTitle?: string;
};
export function DesktopBreadcrumbs({ p, route, detailTitle }: BreadcrumbProps) {
  const session = useContext(SessionRoute);
  return (
    <nav className="bc" aria-label="Caminho">
      {(route === 'credentials' ||
        route === 'adicionar' ||
        route === 'sessao' ||
        (route === 'equipment' && detailTitle)) && (
        <>
          <a
            href="#equipamentos"
            onClick={(e) => {
              e.preventDefault();
              p.navigate('equipment');
            }}
          >
            Equipamentos
          </a>
          <span aria-hidden="true">›</span>
        </>
      )}
      {route === 'sessao' && detailTitle && (
        <>
          <a
            href="#equipamentos"
            onClick={(event) => {
              event.preventDefault();
              session?.back();
            }}
          >
            {detailTitle}
          </a>
          <span aria-hidden="true">›</span>
        </>
      )}
      <b aria-current="page">
        {currentLabel(route, detailTitle, session?.target?.login)}
      </b>
    </nav>
  );
}
function currentLabel(
  route: RouteName,
  detailTitle: string | undefined,
  login: string | undefined,
) {
  if (route === 'sessao') return login;
  if (route === 'equipment' && detailTitle) return detailTitle;
  return (
    (
      {
        vitrine: 'Vitrine',
        overview: 'Visão geral',
        settings: 'Configurações',
        credentials: 'Credenciais',
        adicionar: 'Adicionar equipamento',
      } as Partial<Record<RouteName, string>>
    )[route] || 'Equipamentos'
  );
}
