import type { Branding } from '../features/access/desktop-model';
export { Button } from '../shared/ui/Button';
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
