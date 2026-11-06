import { act, renderHook } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { useImportState } from './add-import-state';
import { inventoryImportAPI } from './presentation';
vi.mock('./presentation', () => ({
  inventoryImportAPI: { choose: vi.fn(), review: vi.fn(), confirm: vi.fn() },
}));
const scope = { profile_id: 'company', activation_id: 'active' };
beforeEach(() => vi.resetAllMocks());
it('explains an import that contains headers but no data rows', async () => {
  vi.mocked(inventoryImportAPI.choose).mockResolvedValue({
    code: 'OK',
    ...scope,
    data: {},
  } as never);
  const { result } = renderHook(() => useImportState('company', 'active'));
  await act(async () => {
    await result.current.choose();
  });
  expect(result.current.error).toMatch(/não há linhas/i);
});
it('asks to review the preview when confirmation detects a changed inventory', async () => {
  vi.mocked(inventoryImportAPI.choose).mockResolvedValue({
    code: 'OK',
    ...scope,
    filename: 'fixture.csv',
    data: {
      import_items: [
        {
          status: 'ready',
          row: {
            line: 2,
            name: 'R',
            address: '192.0.2.1',
            port: '22',
            role: 'other',
            block: '',
            credential: '',
          },
        },
      ],
    },
  } as never);
  vi.mocked(inventoryImportAPI.confirm).mockResolvedValue({
    code: 'REVISION_CONFLICT',
    ...scope,
  } as never);
  const { result } = renderHook(() => useImportState('company', 'active'));
  await act(async () => {
    await result.current.choose();
  });
  await act(async () => {
    await result.current.confirm();
  });
  expect(result.current.error).toMatch(/revise a prévia/i);
  expect(result.current.error).not.toMatch(/empresa/i);
});
