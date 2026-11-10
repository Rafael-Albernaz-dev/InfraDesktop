import { describe, expect, it } from 'vitest';
import { formatTickReason } from './network-format';
import { sanitizeVisibleText } from './network-text';
describe('diagnóstico do tick por campo', () => {
  it.each(['tick-frame', 'tick-cpu', 'tick-metric:tx-rate'])(
    'explica %s e reserva o código ao detalhe',
    (reason) => {
      const formatted = formatTickReason(reason);
      expect(formatted.technical).toBe(reason);
      expect(formatted.summary).not.toContain('tick-');
      const visible = sanitizeVisibleText(reason);
      expect(visible.technical).toBe(reason);
      expect(visible.visible).not.toContain('tick-');
    },
  );
});
