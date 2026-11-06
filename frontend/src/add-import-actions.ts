import { id } from './equipment-reading-format';
import type { ImportRow } from './presentation';
import { inventoryImportAPI } from './presentation';

import type { ImportContext } from './add-import-state';
type InventoryImportResult = Awaited<
  ReturnType<typeof inventoryImportAPI.choose>
>;

function importMessage(code: string) {
  if (code === 'IMPORT_PASSWORD_REJECTED')
    return 'O arquivo contém uma coluna ou chave de senha e foi recusado.';
  if (code === 'REVISION_CONFLICT')
    return 'O inventário mudou. Revise a prévia antes de confirmar novamente.';
  if (['IMPORT_FILE_INVALID', 'INVALID_REQUEST'].includes(code))
    return 'Arquivo inválido ou fora dos limites permitidos.';
  return 'Não foi possível concluir. Confira a empresa e tente novamente.';
}
function own(c: ImportContext, r: InventoryImportResult) {
  return r.profile_id === c.profile && r.activation_id === c.activation;
}
function begin(c: ImportContext) {
  if (c.life.current.working) return false;
  c.life.current.working = true;
  c.s.setBusy(true);
  c.s.setError('');
  return true;
}
function finish(c: ImportContext) {
  c.life.current.working = false;
  if (c.life.current.alive) c.s.setBusy(false);
}
function unavailable(c: ImportContext) {
  if (c.life.current.alive) c.s.setError(importMessage('SERVICE_UNAVAILABLE'));
}
function acceptItems(c: ImportContext, r: InventoryImportResult) {
  if (r.code !== 'OK' || !own(c, r) || !r.data?.import_items) {
    c.s.setError(importMessage(r.code));
    return false;
  }
  c.s.setItems(r.data.import_items);
  c.s.setSelected([]);
  c.life.current.operation = id();
  return true;
}
function acceptChosen(c: ImportContext, r: InventoryImportResult) {
  if (!c.life.current.alive || r.code === 'IMPORT_CANCELLED') return;
  if (r.code === 'OK' && own(c, r) && !r.data?.import_items?.length) {
    c.s.setError('Não há linhas de equipamentos no arquivo.');
    return;
  }
  if (acceptItems(c, r)) {
    c.s.setFilename(r.filename || 'Arquivo');
    c.s.setReceipt(undefined);
  }
}
export async function chooseImport(c: ImportContext) {
  if (!begin(c)) return;
  try {
    acceptChosen(
      c,
      await inventoryImportAPI.choose(
        c.profile,
        c.activation,
        c.s.block,
        c.s.credential,
      ),
    );
  } catch {
    unavailable(c);
  } finally {
    finish(c);
  }
}
export async function reviewImport(c: ImportContext, rows: ImportRow[]) {
  if (!begin(c)) return;
  try {
    const r = await inventoryImportAPI.review(c.profile, c.activation, rows);
    if (c.life.current.alive) acceptItems(c, r);
  } catch {
    unavailable(c);
  } finally {
    finish(c);
  }
}
export async function confirmImport(c: ImportContext) {
  if (!begin(c)) return;
  try {
    const r = await inventoryImportAPI.confirm(
      c.profile,
      c.activation,
      c.life.current.operation,
      c.s.items.map((item) => item.row),
    );
    if (!c.life.current.alive) return;
    if (r.code !== 'OK' || !own(c, r) || !r.data?.import_receipt) {
      c.s.setError(importMessage(r.code));
      return;
    }
    c.s.setReceipt(r.data.import_receipt);
  } catch {
    unavailable(c);
  } finally {
    finish(c);
  }
}
export function resetImport(c: ImportContext) {
  c.s.setItems([]);
  c.s.setFilename('');
  c.s.setReceipt(undefined);
  c.s.setSelected([]);
  c.s.setError('');
  c.life.current.operation = id();
}
