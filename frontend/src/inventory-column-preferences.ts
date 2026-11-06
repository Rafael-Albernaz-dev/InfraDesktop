import { useEffect,useRef,useState } from 'react';
import { defaultColumns } from './inventory-columns';
import { access } from './presentation';


export function useColumnPreferences() {
  const [visible, setVisible] = useState<string[]>(defaultColumns);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const pending = useRef(false);
  const generation = useRef(0);
  useEffect(() => {
    let alive = true;
    const request = generation.current;
    void (async () => {
      try {
        const result = await access.status();
        if (
          alive &&
          request === generation.current &&
          result.code === 'OK' &&
          result.columns?.length
        )
          setVisible(result.columns);
      } catch {
        /* the normal service banner reports unavailable IPC */
      }
    })();
    return () => {
      alive = false;
    };
  }, []);
  async function save(next: string[]) {
    if (pending.current) return;
    pending.current = true;
    generation.current++;
    setBusy(true);
    setError('');
    try {
      const result = await access.setColumns(next);
      if (result.code !== 'OK') throw new Error(result.code);
      setVisible(result.columns || next);
    } catch {
      setError('Não foi possível salvar as colunas. Tente novamente.');
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return { visible, busy, error, save };
}