import { useCallback, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { equipmentAPI as api } from './equipment-api';

import { NetworkPulse } from './network-pulse-context';

function useNetworkPulseRead(p: {
  profile: string;
  activation: string;
  connected: boolean;
  doc?: Document;
  readers: number;
  publish: (pulse: string) => void;
}) {
  const { profile, activation, connected, doc, readers, publish } = p;
  useEffect(() => {
    if (!connected || !profile || !doc || readers) return;
    let generation = 0,
      pending = false,
      stopped = false;
    const read = async () => {
      if (stopped || pending || doc.visibilityState === 'hidden') return;
      pending = true;
      const current = ++generation;
      try {
        const result = await api.list(profile, activation, '', 0, false);
        if (
          !stopped &&
          current === generation &&
          result.code === 'OK' &&
          result.data
        )
          publish(result.data.pulse_network || '');
      } catch {
        // Service availability is owned by the authenticated session.
      } finally {
        if (current === generation) pending = false;
      }
    };
    const visibility = () => {
      ++generation;
      pending = false;
      void read();
    };
    void read();
    const timer = setInterval(() => void read(), 5000);
    doc.addEventListener('visibilitychange', visibility);
    return () => {
      stopped = true;
      clearInterval(timer);
      doc.removeEventListener('visibilitychange', visibility);
    };
  }, [profile, activation, connected, doc, readers, publish]);
}

export function NetworkPulseProvider(p: {
  profile: string;
  activation: string;
  connected: boolean;
  doc?: Document;
  children: ReactNode;
}) {
  const [reading, setReading] = useState({ key: '', pulse: '' });
  const [readers, setReaders] = useState(0);
  const key = `${p.profile}|${p.activation}`;
  const publish = useCallback(
    (pulse: string) => setReading({ key, pulse }),
    [key],
  );
  const register = useCallback(() => {
    setReaders((n) => n + 1);
    return () => setReaders((n) => n - 1);
  }, []);
  useNetworkPulseRead({ ...p, readers, publish });
  return (
    <NetworkPulse.Provider
      value={{
        pulse: reading.key === key ? reading.pulse : '',
        publish,
        register,
      }}
    >
      {p.children}
    </NetworkPulse.Provider>
  );
}

const NETWORK_BADGES: Record<
  string,
  { state: string; text: string; title: string }
> = {
  ok: {
    state: 'ok',
    text: 'Conectado à rede',
    title: 'Equipamentos desta empresa respondendo',
  },
  no_response: {
    state: 'sem',
    text: 'Sem acesso à rede',
    title:
      'Nenhum equipamento desta empresa respondeu. Verifique a rede ou a VPN deste computador.',
  },
  probe_blocked: {
    state: 'sem',
    text: 'Ping sem permissão',
    title:
      'Este computador não tem permissão para enviar ping. Estado indisponível.',
  },
  stopped: {
    state: 'parado',
    text: 'Serviço parado',
    title:
      'O serviço do InfraDesktop não está respondendo; os dados não se atualizam.',
  },
};
export function NetworkBadge({ connected }: { connected: boolean }) {
  const { pulse } = useContext(NetworkPulse);
  const badge =
    NETWORK_BADGES[connected ? pulse : 'stopped'] || NETWORK_BADGES.ok;
  return (
    <span
      className={`selo-rede ${badge.state}`}
      role="status"
      aria-live="polite"
      title={badge.title}
    >
      {badge.text}
    </span>
  );
}
