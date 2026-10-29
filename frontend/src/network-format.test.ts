import { describe, expect, it } from 'vitest';
import {
  exactRate,
  exactVolume,
  formatAtScale,
  formatDuration,
  formatRange,
  formatRate,
  formatVolume,
  networkScale,
} from './network-format';

describe('L06T3 units and exact audit values', () => {
  it.each([
    [1023, '1.023 B'],
    [1024, '1,0 KB'],
    [1048575, '1,0 MB'],
    [33832960, '32,3 MB'],
    [1024 ** 3, '1,0 GB'],
    [1024 ** 4, '1,0 TB'],
    [1024 ** 5, '1,0 PB'],
  ])('volume boundary %s', (n, text) => expect(formatVolume(n)).toBe(text));
  it.each([
    [999, '999 bps'],
    [1000, '1 Kbps'],
    [999999, '1 Mbps'],
    [1000000, '1 Mbps'],
    [1250000, '1,25 Mbps'],
    [100000000, '100 Mbps'],
    [450000000, '450 Mbps'],
    [1000000000, '1 Gbps'],
    [1000000000000, '1 Tbps'],
  ])('rate boundary %s', (n, text) => expect(formatRate(n)).toBe(text));
  it.each([
    [59, '59 s'],
    [60, '1 min'],
    [86399, '23 h 59 min'],
    [86400, '1 d'],
    [45, '45 s'],
    [130, '2 min 10 s'],
    [11700, '3 h 15 min'],
    [1569600, '18 d 4 h'],
  ])('duration boundary %s', (n, text) => expect(formatDuration(n)).toBe(text));
  it.each([undefined, null, -1, NaN, Infinity, 'Indisponível'])(
    'missing/invalid %s stays unavailable',
    (n) => {
      expect(formatVolume(n)).toBe('Indisponível');
      expect(formatRate(n)).toBe('Indisponível');
      expect(formatDuration(n)).toBe('Indisponível');
    },
  );
  it('interval and graph use maximum scale, with raw values preserved', () => {
    expect(formatRange({ min: 950000, max: 1200000 }, 'rate')).toBe(
      '0,95–1,2 Mbps',
    );
    expect(formatRange({ min: 1000000, max: 1000001 }, 'rate')).toBe(
      '1–1,01 Mbps',
    );
    expect(formatRange({ min: -1, max: 100 }, 'rate')).toBe('Indisponível');
    const small = networkScale(900000, 'rate'),
      grown = networkScale(1200000, 'rate');
    expect(formatAtScale(500000, small)).toBe('500 Kbps');
    expect(formatAtScale(500000, grown)).toBe('0,5 Mbps');
    expect(exactVolume('33832960')).toBe('33.832.960 bytes');
    expect(exactRate(100000000)).toBe('100.000.000 bit/s');
    const raw = [1000000000, 950000, 1000000];
    expect([...raw].sort((a, b) => a - b).map(formatRate)).toEqual([
      '950 Kbps',
      '1 Mbps',
      '1 Gbps',
    ]);
    expect(raw).toEqual([1000000000, 950000, 1000000]);
  });
});
import { formatRouterDuration } from './network-format';

it('L07 formats observed wireless last-activity milliseconds through the common duration formatter', () => {
  expect(formatRouterDuration('90ms')).toBe('90 ms');
  expect(formatRouterDuration('3s50ms')).toBe('3 s');
  expect(formatRouterDuration('1d22h27m58s')).toBe('1 d 22 h');
  expect(formatRouterDuration('unavailable')).toBe('Indisponível');
});
