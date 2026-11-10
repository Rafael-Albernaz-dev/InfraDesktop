import { RadioRoles } from '../src/features/radio/radio-contract';

let credentialReady = Promise.resolve();
export function credentialSceneReady() {
  return credentialReady;
}
export function prepareCredentialScene(visualScene: string) {
  if (!['credencial-editar', 'credencial-excluir'].includes(visualScene))
    return;
  credentialReady = new Promise<void>((resolve) => {
    let opened = false;
    const observer = new MutationObserver(() => {
      if (!opened) {
        const row = [...document.querySelectorAll('tr')].find(
          (item) => item.querySelector('td')?.textContent === 'sem-uso',
        );
        if (row) {
          opened = true;
          row.click();
        }
        return;
      }
      const dialog = document.querySelector('[role="dialog"]');
      if (!dialog) return;
      if (
        visualScene === 'credencial-excluir' &&
        dialog.getAttribute('aria-label') !== 'Excluir credencial?'
      ) {
        const button = [...dialog.querySelectorAll('button')].find(
          (item) => item.textContent === 'Excluir credencial',
        );
        button?.click();
        return;
      }
      observer.disconnect();
      resolve();
    });
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
    });
  });
}

export function visualCredentials(visualScene: string) {
  const credentials = [
    {
      id: 'concentrator',
      alias: 'leitura-concentrador',
      user: 'leitura-01',
      revision: 1,
      links: 1,
    },
    {
      id: 'radio',
      alias: 'leitura-radio',
      user: 'leitura-02',
      revision: 1,
      links: 3,
    },
    {
      id: RadioRoles.ptp,
      alias: 'leitura-ptp',
      user: 'leitura-03',
      revision: 1,
      links: 1,
    },
  ];
  if (
    visualScene === 'credencial-editar' ||
    visualScene === 'credencial-excluir'
  ) {
    credentials.push({
      id: 'unused',
      alias: 'sem-uso',
      user: 'x',
      revision: 1,
      links: 0,
    });
  }
  return credentials;
}
