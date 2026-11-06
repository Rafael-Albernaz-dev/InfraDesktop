import { useEffect,useRef,useState } from 'react';
import { id } from './equipment-reading-format';
import type { ImportItem,ImportRow } from './presentation';
import { inventoryImportAPI } from './presentation';

type Receipt = { created: number; ignored: number };
export function useImportState(profile: string, activation: string) {
  const [items, setItems] = useState<ImportItem[]>([]);
  const [filename, setFilename] = useState('');
  const [block, setBlock] = useState('');
  const [credential, setCredential] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [receipt, setReceipt] = useState<Receipt>();
  const [selected, setSelected] = useState<number[]>([]);
  const operation = useRef(id());
  const alive = useRef(true);
  const working = useRef(false);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const own = (r: { profile_id: string; activation_id: string }) =>
    r.profile_id === profile && r.activation_id === activation;
  const message = (code: string) =>
    code === 'IMPORT_PASSWORD_REJECTED'
      ? 'O arquivo contém uma coluna ou chave de senha e foi recusado.'
      : code === 'REVISION_CONFLICT'
        ? 'O inventário mudou. Revise a prévia antes de confirmar novamente.'
      : code === 'IMPORT_FILE_INVALID' || code === 'INVALID_REQUEST'
        ? 'Arquivo inválido ou fora dos limites permitidos.'
        : 'Não foi possível concluir. Confira a empresa e tente novamente.';
  const choose = async () => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError('');
    try {
      const r = await inventoryImportAPI.choose(
        profile,
        activation,
        block,
        credential,
      );
      if (!alive.current || r.code === 'IMPORT_CANCELLED') return;
      if (r.code === 'OK' && own(r) && !r.data?.import_items?.length) {
        setError('Não há linhas de equipamentos no arquivo.');
        return;
      }
      if (r.code !== 'OK' || !own(r) || !r.data?.import_items) {
        setError(message(r.code));
        return;
      }
      setItems(r.data.import_items);
      setFilename(r.filename || 'Arquivo');
      setReceipt(undefined);
      setSelected([]);
      operation.current = id();
    } catch {
      if (alive.current) setError(message('SERVICE_UNAVAILABLE'));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  };
  const review = async (rows: ImportRow[]) => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError('');
    try {
      const r = await inventoryImportAPI.review(profile, activation, rows);
      if (!alive.current) return;
      if (r.code !== 'OK' || !own(r) || !r.data?.import_items) {
        setError(message(r.code));
        return;
      }
      setItems(r.data.import_items);
      setSelected([]);
      operation.current = id();
    } catch {
      if (alive.current) setError(message('SERVICE_UNAVAILABLE'));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  };
  const confirm = async () => {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError('');
    try {
      const r = await inventoryImportAPI.confirm(
        profile,
        activation,
        operation.current,
        items.map((item) => item.row),
      );
      if (!alive.current) return;
      if (r.code !== 'OK' || !own(r) || !r.data?.import_receipt) {
        setError(message(r.code));
        return;
      }
      setReceipt(r.data.import_receipt);
    } catch {
      if (alive.current) setError(message('SERVICE_UNAVAILABLE'));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  };
  const reset = () => {
    setItems([]);
    setFilename('');
    setReceipt(undefined);
    setSelected([]);
    setError('');
    operation.current = id();
  };
  const update = (line: number, patch: Partial<ImportRow>) =>
    void review(
      items.map((item) =>
        item.row.line === line ? { ...item.row, ...patch } : item.row,
      ),
    );
  return {
    items,
    filename,
    block,
    setBlock,
    credential,
    setCredential,
    busy,
    error,
    setError,
    receipt,
    selected,
    setSelected,
    choose,
    review,
    confirm,
    reset,
    update,
  };
}
