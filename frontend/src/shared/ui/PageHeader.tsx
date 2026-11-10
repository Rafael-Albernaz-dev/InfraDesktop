import type { ReactNode } from 'react';

export function PageHeader({
  title,
  meta,
  children,
}: {
  title: string;
  meta?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="head">
      <div className="grow">
        <h1>{title}</h1>
        {meta && <div className="meta">{meta}</div>}
      </div>
      {children}
    </div>
  );
}
