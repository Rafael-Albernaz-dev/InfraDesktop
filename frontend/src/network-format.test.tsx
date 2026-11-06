import { render,screen } from '@testing-library/react';
import { describe,expect,it } from 'vitest';
import {
TechnicalDetail,
exactRate,
exactVolume,
formatAbsent,
formatAge,
formatAtScale,
formatDateWithAge,
formatDeviceModel,
formatDuration,
formatEquipmentRole,
formatFirmwareVersion,
formatIPAddress,
formatLinkOrigin,
formatMacAddress,
formatRadioFamily,
formatRadioMode,
formatRadioQuality,
formatRange,
formatRate,
formatRouterDuration,
formatTextOrAbsent,
formatTickReason,
formatTrustOrigin,
formatVolume,
networkScale,
sanitizeVisibleText,
} from './network-format';
import { RadioFamilies,RadioRoles } from './radio-contract';
const absent = 'Não informado nesta leitura';

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
  it.each([undefined, null, -1, NaN, Infinity, absent])(
    'missing/invalid %s stays unavailable',
    (n) => {
      expect(formatVolume(n)).toBe(absent);
      expect(formatRate(n)).toBe(absent);
      expect(formatDuration(n)).toBe(absent);
    },
  );
  it('interval and graph use maximum scale, with raw values preserved', () => {
    expect(formatRange({ min: 950000, max: 1200000 }, 'rate')).toBe(
      '0,95–1,2 Mbps',
    );
    expect(formatRange({ min: 1000000, max: 1000001 }, 'rate')).toBe(
      '1–1,01 Mbps',
    );
    expect(formatRange({ min: -1, max: 100 }, 'rate')).toBe(absent);
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

it('L07 formats observed wireless last-activity milliseconds through the common duration formatter', () => {
  expect(formatRouterDuration('90ms')).toBe('90 ms');
  expect(formatRouterDuration('3s50ms')).toBe('3 s');
  expect(formatRouterDuration('1d22h27m58s')).toBe('1 d 22 h');
  expect(formatRouterDuration('unavailable')).toBe(absent);
});

describe('unified absence, age, metadata and sanitization formatters', () => {
  it('formats absent values with explicit short reason and rejects bare dashes or NaN/undefined', () => {
    expect(formatAbsent('não coletado')).toBe(`${absent} (não coletado)`);
    expect(formatTextOrAbsent('', 'SSID não exposto')).toBe(
      `${absent} (SSID não exposto)`,
    );
    expect(formatTextOrAbsent('—', 'sem leitura')).toBe(
      `${absent} (sem leitura)`,
    );
    expect(formatTextOrAbsent('undefined', 'sem leitura')).toBe(
      `${absent} (sem leitura)`,
    );
    expect(formatTextOrAbsent('NaN', 'sem leitura')).toBe(
      `${absent} (sem leitura)`,
    );
  });

  it('formats data age in pt-BR relative minutes, hours and days', () => {
    const now = Date.parse('2026-11-03T14:10:00Z');
    expect(formatAge('2026-11-03T14:09:30Z', now)).toBe('agora');
    expect(formatAge('2026-11-03T14:07:00Z', now)).toBe('há 3 min');
    expect(formatAge('2026-11-03T12:10:00Z', now)).toBe('há 2 h');
    expect(formatAge('2026-11-01T14:10:00Z', now)).toBe('há 2 d');
    expect(formatDateWithAge('2026-11-03T14:07:00Z', now)).toContain(
      '(há 3 min)',
    );
    expect(formatDateWithAge('', now)).toBe(`${absent} (sem horário)`);
  });

  it('formats MAC/BSSID, IP, model, firmware, roles and radio families consistently', () => {
    expect(formatMacAddress('aa-bb-cc-dd-ee-ff')).toBe('AA:BB:CC:DD:EE:FF');
    expect(formatMacAddress('')).toBe(`${absent} (MAC não coletado)`);
    expect(formatIPAddress('10.20.30.1', 22)).toBe('10.20.30.1:22');
    expect(formatIPAddress('')).toBe(`${absent} (endereço não informado)`);
    expect(formatDeviceModel('')).toBe(`${absent} (modelo não coletado)`);
    expect(formatFirmwareVersion('')).toBe(`${absent} (firmware não coletado)`);
    expect(formatEquipmentRole(RadioRoles.ap)).toBe('Ponto de acesso (AP)');
    expect(formatEquipmentRole(RadioRoles.cpe)).toBe('Cliente rádio (CPE)');
    expect(formatEquipmentRole('concentrator')).toBe('Concentrador PPPoE');
    expect(formatRadioFamily(RadioFamilies.routerOSWirelessV6)).toBe(
      'MikroTik RouterOS Wireless v6',
    );
    expect(formatRadioFamily(RadioFamilies.intelbrasWOM)).toBe('Intelbras WOM');
    expect(formatRadioFamily(RadioFamilies.standalone)).toBe(
      'Detecção automática',
    );
    expect(formatRadioFamily(RadioFamilies.unsupported)).toBe(
      'Sem driver compatível nesta versão',
    );
    expect(formatRadioMode('station-bridge', '802.11')).toBe(
      'Estação (station-bridge) · 802.11',
    );
    expect(formatRadioQuality('PARTIAL')).toBe('Leitura parcial');
    expect(formatLinkOrigin('sugerido-confirmado')).toBe(
      'Sugestão confirmada pelo operador',
    );
    expect(formatTrustOrigin('automatic-tofu')).toBe(
      'Aceita no primeiro acesso (TOFU)',
    );
  });

  it('sanitizes internal codes, raw errors, paths and secrets into human text + collapsed technical detail', () => {
    const tick = formatTickReason('tick-parse');
    expect(tick.summary).not.toContain('tick-parse');
    expect(tick.action.length).toBeGreaterThan(10);
    expect(tick.technical).toBe('tick-parse');

    const rawErr = sanitizeVisibleText(
      'dial tcp 10.0.0.1:22: connection refused (password=segredo123 /home/toru/key)',
    );
    expect(rawErr.visible).not.toContain('connection refused');
    expect(rawErr.visible).not.toContain('segredo123');
    expect(rawErr.visible).not.toContain('/home/toru');
    expect(rawErr.technical).toContain('password=[oculto]');
    expect(rawErr.technical).toContain('[caminho-local]');

    render(<TechnicalDetail detail={rawErr.technical} />);
    expect(screen.getByText('Detalhe técnico')).toBeInTheDocument();
  });

  it('DOM sanitization: internal tokens never leak to DOM text outside <details>', () => {
    const rawTechnical = [
      'Falha no tick-parse: driver',
      RadioFamilies.routerOSWirelessV6,
      'com fallback',
      RadioFamilies.standalone,
      RadioFamilies.unsupported,
    ].join(' ');
    const sanitized = sanitizeVisibleText(rawTechnical);

    const { container } = render(
      <div data-testid="container">
        <p>{sanitized.visible}</p>
        <p>{formatRadioFamily(RadioFamilies.routerOSWirelessV6)}</p>
        <p>{formatRadioFamily(RadioFamilies.standalone)}</p>
        <p>{formatRadioFamily(RadioFamilies.unsupported)}</p>
        <p>{formatAbsent('não coletado')}</p>
        <p>{formatTextOrAbsent('undefined')}</p>
        <p>{formatTextOrAbsent('NaN')}</p>
        <TechnicalDetail detail={sanitized.technical} />
      </div>,
    );

    const clone = container.cloneNode(true) as HTMLElement;
    clone.querySelectorAll('details').forEach((el) => el.remove());
    const outsideText = clone.textContent || '';

    const leakedTokens = [
      'tick-parse',
      RadioFamilies.routerOSWirelessV6,
      RadioFamilies.standalone,
      RadioFamilies.unsupported,
      'undefined',
      'NaN',
    ];

    for (const token of leakedTokens) {
      expect(outsideText).not.toContain(token);
    }

    const details = container.querySelector('details');
    expect(details).toBeInTheDocument();
    expect(details?.textContent).toContain('tick-parse');
  });
});
