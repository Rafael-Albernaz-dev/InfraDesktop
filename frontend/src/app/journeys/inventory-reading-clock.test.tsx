import { render } from '@testing-library/react';
import { expect, it } from 'vitest';
import type { Device } from '../../features/equipment/equipment-api';
import { columnById } from '../../features/inventory/inventory-columns';

const device = (atraso: number) =>
  ({
    id: 'a',
    active: true,
    live: {
      cpu: 7,
      clients: 3,
      uptime_seconds: 60,
      at: new Date(Date.now() - atraso).toISOString(),
      fault: '',
    },
  }) as unknown as Device;

const idade = (d: Device, now: number) => {
  const { container, unmount } = render(
    <>
      {columnById('last_reading')!.cell({ device: d, now, credentials: [] })}
    </>,
  );
  const text = container.textContent;
  unmount();
  return text;
};

it('reading newer than the table clock never shows an invalid age', () => {
  expect(idade(device(5000), Date.now() - 25000)).toMatch(/^há [5-6] s$/);
});

it('age follows the real clock, not the 30 second table clock', () => {
  expect(idade(device(40000), Date.now() - 29000)).toMatch(/^há 4[0-1] s$/);
});

it('reading from the future never shows a negative or invalid age', () => {
  expect(idade(device(-5000), Date.now())).toBe('há 0 s');
});
