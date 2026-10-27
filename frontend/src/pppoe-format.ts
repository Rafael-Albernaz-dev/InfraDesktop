import type { PPInterval } from './presentation';

export function formatPPUptime(value: PPInterval): string {
  if (!value.known) return 'Indisponível';
  let seconds = Math.floor(value.min / 1e9);
  const parts: string[] = [];
  for (const [unit, size] of [
    ['d', 86400],
    ['h', 3600],
    ['min', 60],
    ['s', 1],
  ] as const) {
    const count = Math.floor(seconds / size);
    seconds %= size;
    if (count || (size === 1 && !parts.length)) parts.push(`${count} ${unit}`);
  }
  return parts.join(' ');
}
