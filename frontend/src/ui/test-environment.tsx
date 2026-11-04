import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
vi.stubGlobal('matchMedia', (query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener() {},
  removeListener() {},
  addEventListener() {},
  removeEventListener() {},
  dispatchEvent: () => false,
}));
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);
const context = new Proxy(
  {
    measureText: () => ({ width: 20 }),
    createLinearGradient: () => ({ addColorStop() {} }),
  },
  {
    get: (object, key) =>
      key in object ? object[key as keyof typeof object] : () => {},
  },
);
Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
  value: () => context,
});
vi.stubGlobal(
  'Path2D',
  class {
    moveTo() {}
    lineTo() {}
    rect() {}
    arc() {}
    closePath() {}
    bezierCurveTo() {}
  },
);
afterEach(cleanup);
