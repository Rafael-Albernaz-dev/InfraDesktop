import { useEffect, useRef, useState } from 'react';
import {
  chooseImport,
  confirmImport,
  resetImport,
  reviewImport,
} from './add-import-actions';
import { id } from './equipment-reading-format';
import type { ImportItem, ImportRow } from './presentation';

type Receipt = { created: number; ignored: number };
function useImportStore() {
  const [items, setItems] = useState<ImportItem[]>([]);
  const [filename, setFilename] = useState('');
  const [block, setBlock] = useState('');
  const [credential, setCredential] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [receipt, setReceipt] = useState<Receipt>();
  const [selected, setSelected] = useState<number[]>([]);
  return {
    items,
    setItems,
    filename,
    setFilename,
    block,
    setBlock,
    credential,
    setCredential,
    busy,
    setBusy,
    error,
    setError,
    receipt,
    setReceipt,
    selected,
    setSelected,
  };
}
export type ImportStore = ReturnType<typeof useImportStore>;
export type ImportContext = {
  profile: string;
  activation: string;
  s: ImportStore;
  life: React.MutableRefObject<{
    operation: string;
    alive: boolean;
    working: boolean;
  }>;
};
export function useImportState(profile: string, activation: string) {
  const s = useImportStore(),
    life = useRef({ operation: id(), alive: true, working: false });
  useEffect(() => {
    const lifecycle = life.current;
    lifecycle.alive = true;
    return () => {
      lifecycle.alive = false;
    };
  }, []);
  const c: ImportContext = { profile, activation, s, life };
  return {
    items: s.items,
    filename: s.filename,
    block: s.block,
    setBlock: s.setBlock,
    credential: s.credential,
    setCredential: s.setCredential,
    busy: s.busy,
    error: s.error,
    setError: s.setError,
    receipt: s.receipt,
    selected: s.selected,
    setSelected: s.setSelected,
    choose: () => chooseImport(c),
    review: (rows: ImportRow[]) => reviewImport(c, rows),
    confirm: () => confirmImport(c),
    reset: () => resetImport(c),
    update: (line: number, patch: Partial<ImportRow>) =>
      void reviewImport(
        c,
        s.items.map((item) =>
          item.row.line === line ? { ...item.row, ...patch } : item.row,
        ),
      ),
  };
}
