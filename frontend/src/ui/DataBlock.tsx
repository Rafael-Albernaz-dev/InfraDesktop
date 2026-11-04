import type { ReactNode } from 'react';
import { Button } from './Button';

export type DataState =
  | { state: 'unread'; action: () => void; disabled?: boolean }
  | { state: 'loading' }
  | { state: 'refreshing'; content: ReactNode }
  | { state: 'ready'; content: ReactNode }
  | { state: 'empty'; message: string }
  | {
      state: 'failed';
      previous?: ReactNode;
      reason: string;
      technical: string;
      retry: () => void;
      disabled?: boolean;
    }
  | { state: 'not-supported'; resource: string }
  | { state: 'unbuilt'; slice: string };

function StateContent(p: DataState) {
  switch (p.state) {
    case 'unread':
      return (
        <>
          <p>Ainda sem leitura</p>
          <Button onClick={p.action} disabled={p.disabled}>
            Atualizar
          </Button>
        </>
      );
    case 'loading':
      return (
        <div role="status" aria-label="Carregando">
          <div className="skel" />
          <div className="skel medium" />
          <div className="skel short" />
        </div>
      );
    case 'refreshing':
      return (
        <>
          <span className="spin" role="status">
            atualizando…
          </span>
          {p.content}
        </>
      );
    case 'ready':
      return <>{p.content}</>;
    case 'empty':
      return <p>{p.message}</p>;
    case 'failed':
      return (
        <>
          <p role="alert">{p.reason}</p>
          <Button disabled={p.disabled} onClick={p.retry}>
            Tentar novamente
          </Button>
          <details className="det">
            <summary>Detalhe técnico</summary>
            <p>Motivo: {p.technical}</p>
          </details>
          {p.previous}
        </>
      );
    case 'not-supported':
      return <p>Este modelo não informa {p.resource}</p>;
    case 'unbuilt':
      return <p>Ainda não existe ({p.slice})</p>;
  }
}

export function DataBlock({
  title,
  source,
  age = '',
  data,
}: {
  title: string;
  source: string;
  age?: string;
  data: DataState;
}) {
  const showAge = data.state !== 'unread' && data.state !== 'loading' && age;
  return (
    <div className="block" data-state={data.state}>
      <div className="bh">
        <b>{title}</b>
        <span>
          {source}
          {showAge ? ` · ${age}` : ''}
        </span>
      </div>
      <div className="bb">
        <StateContent {...data} />
      </div>
    </div>
  );
}
