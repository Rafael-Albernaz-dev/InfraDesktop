import { act, fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { PPPoE } from './pppoe';
import { equipmentAPI, pppoeAPI } from './presentation';
import { pendingStarts, pendingPPPoE } from './equipment-receipts';
import type { Device } from './equipment-api';
vi.mock('./presentation', () => ({
  equipmentAPI: {
    list: vi.fn(),
    get: vi.fn(),
    credentials: vi.fn(),
    start: vi.fn(),
    inspect: vi.fn(),
  },
  pppoeAPI: {
    current: vi.fn(),
    references: vi.fn(),
    list: vi.fn(),
    start: vi.fn(),
  },
}));
const device = {
  id: 'device',
  name: 'Receipt router',
  revision: 2,
  active: true,
  fingerprint: 'pinned',
  credential_id: 'credential',
} as Device;
const result = { code: 'OK', profile_id: 'c', activation_id: 'a' };
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(equipmentAPI.credentials).mockResolvedValue({
    ...result,
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
    },
  });
  vi.mocked(equipmentAPI.start).mockResolvedValue({
    ...result,
    code: 'SERVICE_UNAVAILABLE',
  });
  vi.mocked(pppoeAPI.start).mockResolvedValue({
    ...result,
    code: 'SERVICE_UNAVAILABLE',
  });
  vi.mocked(pppoeAPI.current).mockResolvedValue({
    ...result,
    data: {
      current: {
        observation_id: 'o',
        count: 0,
        quality: { status: 'complete' },
        provenance: { ended_at: '2026-10-25T00:00:00Z' },
      },
      total: 0,
    },
  } as never);
  vi.mocked(pppoeAPI.references).mockResolvedValue({
    ...result,
    data: { references: [], total: 0 },
  });
  vi.mocked(pppoeAPI.list).mockResolvedValue({
    ...result,
    data: { rows: [], total: 0 },
  });
});
for (const revision of [2, 3]) {
  it(`C01a legacy revision${revision} retires only older receipts across both kinds`, async () => {
    const d = { ...device, revision };
    vi.mocked(equipmentAPI.list).mockResolvedValue({
      ...result,
      data: {
        devices: [d],
        credentials: [],
        next_cursor: 0,
        vault_available: true,
      },
    });
    vi.mocked(equipmentAPI.get).mockResolvedValue({
      ...result,
      data: {
        device: d,
        devices: [],
        credentials: [],
        next_cursor: 0,
        vault_available: true,
      },
    });
    render(
      <Equipment profile="c" activation="a" connected createRequest={0} />,
    );
    await screen.findByRole('heading', { name: 'Receipt router' });
    pendingStarts.clear();
    for (const kind of ['query', 'inspect'])
      for (const r of [2, 3])
        pendingStarts.set(`c|a|${kind}|device|${r}`, `${kind}-revision${r}`);
    await act(async () =>
      fireEvent.click(screen.getByRole('button', { name: 'Conectar' })),
    );
    for (const kind of ['query', 'inspect']) {
      expect(pendingStarts.get(`c|a|${kind}|device|3`)).toBe(
        `${kind}-revision3`,
      );
      expect(pendingStarts.has(`c|a|${kind}|device|2`)).toBe(revision === 2);
    }
    expect(vi.mocked(equipmentAPI.start).mock.calls[0][3]).toBe(
      `query-revision${revision}`,
    );
  });
  it(`C01a PPPoE revision${revision} preserves newer collection IDs and observation mutations`, async () => {
    render(
      <PPPoE
        profile="c"
        activation="a"
        device={{ ...device, revision }}
        back={vi.fn()}
      />,
    );
    await screen.findByText('Nenhuma sessão ativa nesta leitura completa.');
    pendingPPPoE.clear();
    for (const r of [2, 3])
      pendingPPPoE.set(`c|a|device|collect|${r}`, {
        operation: `collection-revision${r}`,
        revision: r,
      });
    pendingPPPoE.set('c|a|device|save|o', { operation: 'save-observation' });
    pendingPPPoE.set('c|a|device|delete|ref', {
      operation: 'delete-reference',
    });
    await act(async () =>
      fireEvent.click(
        screen.getByRole('button', { name: 'Atualizar sessões' }),
      ),
    );
    expect(pendingPPPoE.get('c|a|device|collect|3')?.operation).toBe(
      'collection-revision3',
    );
    expect(pendingPPPoE.has('c|a|device|collect|2')).toBe(revision === 2);
    expect(pendingPPPoE.get('c|a|device|save|o')?.operation).toBe(
      'save-observation',
    );
    expect(pendingPPPoE.get('c|a|device|delete|ref')?.operation).toBe(
      'delete-reference',
    );
    expect(vi.mocked(pppoeAPI.start).mock.calls[0][3]).toBe(
      `collection-revision${revision}`,
    );
  });
}
