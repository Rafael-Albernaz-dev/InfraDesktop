import { emptyOperations } from '../src/desktop-model';
// Separate test entry. Never imported by src/main.tsx or the production bundle.
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Desktop, Dialog, Statusbar } from '../src/desktop';
import type { Page, Branding, Operations } from '../src/desktop';
import { Security } from '../src/security';
import { AccessScreen } from '../src/access-screen';
import fixture from './fixture.json';
import '../src/style.css';
export function Fixture() {
  const q = new URLSearchParams(location.search);
  const variant = q.get('variant') || '';
  const [page, navigate] = useState(Number(q.get('scene')) || 1),
    [theme, setTheme] = useState(q.get('theme') || 'light'),
    [dialog, setDialog] = useState('');
  const [brand, saveBrand] = useState<Branding>({
    name: q.has('alt') ? 'Aurora Telecom' : 'SerraLink Telecom',
    accent: q.has('alt') ? 'violet' : 'teal',
    logo: '',
  });
  const operations: Operations = q.has('empty')
    ? emptyOperations
    : (fixture as Operations);
  const companies = [
    {
      id: 'serra',
      name: brand.name,
      lastAccess: 'Último acesso hoje, 14:08',
      connection: 'VPN disponível',
      connectionStyle: 'good',
      count: 6,
      network: 'VPN · Escritório',
      inventory: '6 equipamentos · 2 concentradores',
      access: 'Salvos neste perfil',
    },
    {
      id: 'vale',
      name: 'ValeNet ISP',
      lastAccess: 'Último acesso em 20 out, 09:42',
      connection: 'Sem rota',
      connectionStyle: 'warn',
      network: 'Sem rota disponível',
      inventory: 'Não consultado',
      access: 'Perfil separado',
      disabled: true,
    },
  ];
  if (page === 1)
    return (
      <div className="app" data-theme={theme} data-page="1">
        <div className="client">
          <AccessScreen
            theme={theme}
            setTheme={setTheme}
            setup
            busy={false}
            connected={variant !== 'unavailable'}
            error={variant === 'invalid' ? 'Usuário ou senha não conferem' : ''}
            initialMode={
              variant === 'register' || variant === 'recover'
                ? variant
                : 'login'
            }
            recovery={
              variant === 'code' ? 'DEMONSTRACAO-SEM-VALOR-DE-RECUPERACAO' : ''
            }
            acknowledge={() => {}}
            initialName="operador"
            submit={() => navigate(2)}
          />
          <Statusbar connected={variant !== 'unavailable'} access />
        </div>
      </div>
    );
  return (
    <Desktop
      page={page as Page}
      navigate={navigate}
      brand={brand}
      account="Operador"
      theme={theme}
      setTheme={setTheme}
      connected
      busy={false}
      companies={variant === 'no-company' ? [] : companies}
      activeProfile={variant === 'no-company' ? '' : 'serra'}
      create={() => setDialog('Nova empresa')}
      enter={() => navigate(3)}
      logout={() => setDialog('Sair do InfraDesktop?')}
      unavailable={setDialog}
      operations={operations}
      saveBrand={async (next) => {
        saveBrand(next);
        return true;
      }}
      security={() => setDialog('Segurança')}
    >
      {variant.startsWith('security') && (
        <Dialog title="Segurança" close={() => {}}>
          <Security
            recovery={
              variant === 'security-code'
                ? 'DEMONSTRACAO-SEM-VALOR-DE-RECUPERACAO'
                : ''
            }
            theme={theme}
            setTheme={setTheme}
            busy={false}
            error={
              variant === 'security-rejected'
                ? 'A senha não confere. Tente novamente.'
                : ''
            }
            uncertain={variant === 'security-unknown'}
            lock={() => {}}
            renew={() => {}}
            acknowledge={() => {}}
          />
        </Dialog>
      )}
      {dialog && (
        <Dialog title={dialog} close={() => setDialog('')}>
          <p>Bancada isolada de apresentação. Sem operação de produto.</p>
        </Dialog>
      )}
    </Desktop>
  );
}
createRoot(document.getElementById('root')!).render(<Fixture />);
