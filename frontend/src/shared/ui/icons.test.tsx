import { render } from '@testing-library/react';
import { expect, it } from 'vitest';
import { Icon } from './icons';

it('a known icon draws its rounded outline paths', () => {
  const { container } = render(<Icon name="Cog6Tooth" />);
  const paths = [...container.querySelectorAll('svg.icon path')];
  expect(paths).toHaveLength(2);
  for (const p of paths) {
    expect(p.getAttribute('stroke-linecap')).toBe('round');
    expect(p.getAttribute('stroke-linejoin')).toBe('round');
  }
});

it('an unknown icon renders an empty outline frame, even for object prototype names', () => {
  for (const name of ['Desconhecido', 'constructor', 'toString', '__proto__']) {
    const { container, unmount } = render(<Icon name={name} />);
    expect(container.querySelector('svg.icon'), name).not.toBeNull();
    expect(container.querySelectorAll('path'), name).toHaveLength(0);
    unmount();
  }
});
