import type { ReactNode } from 'react';

export function StateNotice({
  children,
  error = false,
}: {
  children: ReactNode;
  error?: boolean;
}) {
  return (
    <div className="note" role={error ? 'alert' : 'status'}>
      {children}
    </div>
  );
}
