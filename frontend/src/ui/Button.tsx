import type { ComponentProps } from 'react';

type Props = ComponentProps<'button'> & { kind?: string; label?: string };

export function Button({
  kind = '',
  label,
  className = '',
  type = 'button',
  ...props
}: Props) {
  return (
    <button
      {...props}
      type={type}
      className={`${kind} ${className}`.trim()}
      aria-label={label || props['aria-label']}
    />
  );
}
