import { useEffect, useRef, useState } from 'react';
import type { Branding } from '../../app/desktop-model';

// Retained draft behavior, without a routed personalization screen.
export function useBrandDraft(
  brand: Branding,
  busy: boolean,
  ready: boolean,
  saveBrand?: (b: Branding) => void | boolean | Promise<boolean>,
) {
  const [draft, setDraft] = useState(brand),
    [dirty, setDirty] = useState(false),
    [error, setError] = useState('');
  const { loadingImage, input, readFile, invalidateRead } = useBrandImage({
    brand,
    setDraft,
    setDirty,
    setError,
  });
  const change = (next: Branding) => {
    setDraft(next);
    setDirty(true);
    setError('');
  };
  const cancel = () => {
    invalidateRead();
    setDraft(brand);
    setDirty(false);
    setError('');
  };
  const reset = () => {
    invalidateRead();
    change({ name: '', logo: '', accent: 'teal' });
  };
  const remove = () => {
    invalidateRead();
    change({ ...draft, logo: '' });
  };
  const save = async () => {
    invalidateRead();
    if (await saveBrand?.(draft)) setDirty(false);
  };
  return {
    draft,
    dirty,
    error,
    loadingImage,
    input,
    change,
    readFile,
    cancel,
    reset,
    remove,
    save,
    busy,
    ready,
    canSave: dirty && !!saveBrand && !busy && ready && !loadingImage,
  };
}
function useBrandImage(p: {
  brand: Branding;
  setDraft: React.Dispatch<React.SetStateAction<Branding>>;
  setDirty: (v: boolean) => void;
  setError: (v: string) => void;
}) {
  const { brand, setDraft, setDirty, setError } = p;
  const [loadingImage, setLoadingImage] = useState(false);
  const input = useRef<HTMLInputElement>(null),
    reading = useRef<FileReader | null>(null),
    generation = useRef(0);
  const invalidateRead = () => {
    generation.current++;
    reading.current?.abort();
    reading.current = null;
    setLoadingImage(false);
    if (input.current) input.current.value = '';
  };
  useEffect(
    () => () => {
      generation.current++;
      reading.current?.abort();
    },
    [brand.name, brand.logo, brand.accent],
  );
  const readFile = (file?: File) => {
    invalidateRead();
    if (!file) return;
    if (
      !['image/png', 'image/jpeg', 'image/webp'].includes(file.type) ||
      file.size > 2 * 1024 * 1024
    ) {
      setError('Escolha uma imagem PNG, JPEG ou WebP de até 2 MB.');
      return;
    }
    const reader = new FileReader(),
      current = generation.current;
    reading.current = reader;
    setLoadingImage(true);
    setDirty(true);
    reader.onload = () => {
      if (current !== generation.current) return;
      reading.current = null;
      setLoadingImage(false);
      setDraft((previous) => ({ ...previous, logo: String(reader.result) }));
      setDirty(true);
      setError('');
    };
    reader.onerror = () => {
      if (current === generation.current) {
        reading.current = null;
        setLoadingImage(false);
        setError('Não foi possível ler a imagem. Escolha outra.');
      }
    };
    reader.readAsDataURL(file);
  };
  return { loadingImage, input, readFile, invalidateRead };
}
