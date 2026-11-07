import { render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import type { Device } from './equipment-api';
import { latestSavedReading } from './equipment-reading-format';
import { columns } from './inventory-columns';

const now = new Date('2026-11-07T12:00:00Z').getTime();
const device = {
  id: 'synthetic',
  active: true,
  result: {
    observed_at: new Date(now - 600_000).toISOString(),
    cpu_percent: 2,
    version: '6.49.19',
    uptime: '1h',
  },
  pppoe: { online: 3, offline: 8 },
  live: {
    cpu: 99,
    clients: 42,
    uptime_seconds: 7200,
    at: new Date(now - 30_000).toISOString(),
    fault: '',
  },
} as Device;
function cell(id: string, d = device) {
  return columns
    .find((c) => c.id === id)!
    .cell({ device: d, now, credentials: [] });
}
afterEach(() => vi.useRealTimers());
it('shows live metrics and age while preserving saved system and offline clients', () => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  render(
    <div>
      {cell('cpu')}
      {cell('online')}
      {cell('offline')}
      {cell('uptime')}
      {cell('system')}
      {cell('last_reading')}
    </div>,
  );
  expect(cell('cpu')).toBe('99 %');
  expect(cell('online')).toBe(42);
  expect(cell('offline')).toBe(8);
  expect(cell('system')).toBe('6.49.19');
  expect(latestSavedReading(device).at).toBe(device.live!.at);
  expect(latestSavedReading(device).uptime).toBe('2 h');
});
it('falls back after 180 seconds and leaves lost readings with their age', () => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  const lost = { ...device, live: { ...device.live!, fault: 'lost' } };
  expect(latestSavedReading(lost).cpu).toBe(99);
  render(<>{cell('last_reading', lost)}</>);
  expect(screen.getByText(/há /)).toBeInTheDocument();
  vi.setSystemTime(now + 150_001);
  expect(latestSavedReading(lost).cpu).toBe(2);
  expect(cell('online', lost)).toBe(3);
  expect(latestSavedReading({ ...device, live: undefined }).cpu).toBe(2);
});
it.each([
  ['refused', 'SENHA INCORRETA'],
  ['key_changed', 'CHAVE SSH ALTERADA'],
])('shows %s in the existing error style', (fault, label) => {
  render(<>{cell('last_reading', { ...device, live: { fault } })}</>);
  expect(screen.getByText(label)).toHaveClass('st-off');
});
it('keeps the existing empty reading text and placeholders', () => {
  const empty = { id: 'empty', active: true } as Device;
  expect(cell('online', empty)).toBe('—');
  expect(cell('cpu', empty)).toBe('—');
  expect(cell('uptime', empty)).toBe('—');
  expect(cell('last_reading', empty)).toBe('Ainda sem leitura');
});
