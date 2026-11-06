import { flushSync } from 'react-dom';
import { createRoot } from 'react-dom/client';
import { AppOverlays } from './app-session';

export function prepareSecurityVisual(scene: string, doc: Document) {
  if (scene !== 'seguranca-erro' && scene !== 'seguranca-codigo') return;
  let requested = false;
  const observer = new MutationObserver(() => {
    const app = doc.querySelector<HTMLElement>(
      '.app[data-route="settings"]',
    );
    if (app) {
      observer.disconnect();
      const host = doc.createElement('div');
      app.appendChild(host);
      const root = createRoot(host);
      flushSync(() =>
        root.render(
          <AppOverlays
            dialog="Segurança"
            close={() => {}}
            recovery={scene === 'seguranca-codigo' ? 'XXXX-XXXX-XXXX' : ''}
            busy={false}
            error={scene === 'seguranca-erro' ? 'SENHA INCORRETA' : ''}
            recoveryUncertain={false}
            securityLockedRef={{ current: false }}
            workingRef={{ current: false }}
            perform={async () => false}
            setRecovery={() => {}}
            setRecoveryUncertain={() => {}}
            setPage={() => {}}
          />,
        ),
      );
      return;
    }
    const button = doc.querySelector<HTMLButtonElement>(
      'button[aria-label="Configurações"]',
    );
    if (button && !requested) {
      requested = true;
      button.click();
    }
  });
  observer.observe(doc.documentElement, {
    childList: true,
    subtree: true,
  });
}
