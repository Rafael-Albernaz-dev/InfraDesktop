import { useEffect, useState } from 'react';
import { readCredentialUsage } from './credential-usage-reader';
import type { CredentialUsage } from './credential-usage-reader';

export function useCredentialUsage(profile: string, activation: string) {
  const key = [profile, activation].join('|');
  const [state, setState] = useState<{
    key: string;
    names: CredentialUsage | undefined;
  }>();
  useEffect(() => {
    let alive = true;
    void readCredentialUsage(profile, activation)
      .then((names) => {
        if (alive) setState({ key, names });
      })
      .catch(() => {
        if (alive) setState({ key, names: undefined });
      });
    return () => {
      alive = false;
    };
  }, [profile, activation, key]);
  return state?.key === key ? state.names : undefined;
}
