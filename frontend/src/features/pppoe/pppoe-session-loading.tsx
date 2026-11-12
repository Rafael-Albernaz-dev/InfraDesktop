import { useState } from 'react';
import { Button } from '../../shared/ui/Button';
import { DataBlock } from '../../shared/ui/DataBlock';
import { Tabs } from '../../shared/ui/Tabs';

type SessionTab = 'traffic' | 'information' | 'link' | 'history';
export function SessionLoading(p: {
  tab?: SessionTab;
  onTab?: (tab: SessionTab) => void;
  login?: string;
  device: string;
  error?: string;
  back: () => void;
}) {
  const [tab, setTab] = useState<SessionTab>('traffic');
  return (
    <section
      className="session-screen"
      aria-label="Detalhes da sessão"
      aria-busy={!p.error}
    >
      <div className="head">
        <div className="grow">
          <h1>{p.error ? 'Sessão' : p.login || 'Sessão'}</h1>
          <div className="meta">
            <span
              className={p.error ? undefined : 'spin'}
              role={p.error ? 'alert' : 'status'}
            >
              {p.error || 'Carregando sessão…'}
            </span>
          </div>
        </div>
        <Button onClick={p.back}>Voltar</Button>
      </div>
      {!p.error && (
        <>
          <Tabs
            label="Detalhes da sessão"
            items={[
              ['traffic', 'Tráfego'],
              ['information', 'Informações'],
              ['link', 'Enlace'],
              ['history', 'Histórico'],
            ]}
            value={p.tab || tab}
            onChange={p.onTab || setTab}
          />
          <div className="session-grid">
            {['Informações', 'Tráfego'].map((title) => (
              <DataBlock
                key={title}
                title={title}
                source={`SSH (${p.device})`}
                data={{ state: 'loading' }}
              />
            ))}
          </div>
        </>
      )}
    </section>
  );
}
