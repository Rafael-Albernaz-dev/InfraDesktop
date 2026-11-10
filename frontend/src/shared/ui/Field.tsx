import { useId } from 'react';
import type { ComponentProps, ReactNode } from 'react';

export function Field({
  label,
  children,
  layout = 'access',
  ...props
}: ComponentProps<'input'> & { label: ReactNode; layout?: 'access' | 'form' }) {
  const generated = useId();
  const id = props.id || generated;
  return (
    <label
      className={layout === 'form' ? 'form-field' : 'lg-field'}
      htmlFor={id}
    >
      <span>{label}</span>
      <input {...props} id={id} />
      {children}
    </label>
  );
}
