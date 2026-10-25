import type { ReactNode } from 'react';
import type { Branding } from './desktop-model';
export function Button({
  children,
  onClick,
  kind = '',
  disabled = false,
  label,
}: {
  children: ReactNode;
  onClick?: () => void;
  kind?: string;
  disabled?: boolean;
  label?: string;
}) {
  return (
    <button
      type="button"
      className={'btn ' + kind}
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
    >
      {children}
    </button>
  );
}
export function Logo({ brand, size = '' }: { brand: Branding; size?: string }) {
  const initials =
    brand.name
      .trim()
      .split(/\s+/)
      .map((x) => Array.from(x)[0] || '')
      .join('')
      .slice(0, 2)
      .toUpperCase() || '—';
  return (
    <span className={'logo ' + size}>
      {brand.logo ? <img src={brand.logo} alt="" /> : initials}
    </span>
  );
}
