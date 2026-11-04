import { Fragment } from 'react';
import type { ReactNode } from 'react';

export function KeyValues({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className="kv">
      {rows.map(([label, value]) => (
        <Fragment key={label}>
          <dt>{label}</dt>
          <dd>{value ?? 'Não informado nesta leitura'}</dd>
        </Fragment>
      ))}
    </dl>
  );
}
