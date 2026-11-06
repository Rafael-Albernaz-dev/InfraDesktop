import { useEffect } from 'react';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Credential } from './equipment-api';
export function useCredentialsCatalogEffect(p: {
  profile: string;
  activation: string;
  connected: boolean;
  credentialRefresh: number;
  setCredentialDefaults: (v: Record<string, string>) => void;
  setCredentialCatalog: (v: { key: string; value: Credential[] }) => void;
  setCredentialError: (v: string) => void;
}) {
  useEffect(() => {
    if (!p.profile || !p.connected) return;
    let stopped = false;
    void (async () => {
      const all: Credential[] = [];
      let page = 0;
      do {
        const r = await api.credentials(p.profile, p.activation, page);
        if (stopped) return;
        if (r.code !== 'OK' || !r.data) throw new Error(r.code);
        all.push(...(r.data.credentials || []));
        p.setCredentialDefaults(r.data.credential_defaults || {});
        page = r.data.next_cursor;
      } while (page);
      p.setCredentialCatalog({
        key: [p.profile, p.activation].join('|'),
        value: all,
      });
      p.setCredentialError('');
    })().catch(() => {
      if (!stopped)
        p.setCredentialError(equipmentMessage('SERVICE_UNAVAILABLE'));
    });
    return () => {
      stopped = true;
    };
  }, [
    p.profile,
    p.activation,
    p.connected,
    p.credentialRefresh,
    p.setCredentialDefaults,
    p.setCredentialCatalog,
    p.setCredentialError,
  ]);
}
