import { useEffect, useRef, useState } from 'react';
import type { Branding } from './desktop-model';
import type { DesktopProps } from './desktop-model';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';
import { Field } from './ui/Field';
import { ProductBrand } from './ui/ProductBrand';
export function CompanyList({ p }: { p: DesktopProps }) {
  return (
    <div className="solo companies-page">
      <div className="lg-brand">
        <ProductBrand />
      </div>
      <div className="card">
        <div className="toolbar">
          <h1 className="grow">Empresas</h1>
          <Button
            kind="primary"
            disabled={p.busy || !p.connected}
            onClick={p.create}
          >
            Criar empresa
          </Button>
        </div>
        <DataTable>
          <thead>
            <tr>
              <th>Empresa</th>
              <th>Equipamentos</th>
            </tr>
          </thead>
          <tbody>
            {p.companies.map((company) => (
              <tr
                key={company.id}
                className="click"
                tabIndex={0}
                onClick={() => !company.disabled && p.enter(company.id)}
                onKeyDown={(e) => {
                  if (
                    (e.key === 'Enter' || e.key === ' ') &&
                    !company.disabled
                  ) {
                    e.preventDefault();
                    p.enter(company.id);
                  }
                }}
              >
                <td>
                  {company.name}
                  {company.id === p.activeProfile && <span> · Atual</span>}
                </td>
                <td>
                  {company.count === undefined
                    ? 'Não informado nesta leitura'
                    : company.count
                      ? `${company.count} equipamentos`
                      : 'Nenhum equipamento'}
                </td>
              </tr>
            ))}
            {!p.companies.length && (
              <tr>
                <td colSpan={2}>Nenhuma empresa cadastrada</td>
              </tr>
            )}
          </tbody>
        </DataTable>
      </div>
      {!p.companies.length && (
        <Button onClick={() => p.navigate('settings')}>Configurações</Button>
      )}
      {p.error && <p role="alert">{p.error}</p>}
    </div>
  );
}
export function Settings({ p }: { p: DesktopProps }) {
  return (
    <>
      <h1>Configurações</h1>
      <div className="card settings-card">
        <h2>Senha</h2>
        <Field
          layout="form"
          label="Senha atual"
          type="password"
          autoComplete="current-password"
        />
        <Field
          layout="form"
          label="Nova senha"
          type="password"
          autoComplete="new-password"
        />
        <Field
          layout="form"
          label="Confirmar nova senha"
          type="password"
          autoComplete="new-password"
        />
        <Button kind="primary" disabled title="Ainda não existe (L-CONTA)">
          Alterar senha
        </Button>
      </div>
      <div className="card settings-card">
        <h2>Código de recuperação</h2>
        <Button onClick={p.security} disabled={p.busy || !p.connected}>
          Gerar novo código
        </Button>
      </div>
    </>
  );
}

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
  const change = (next: Branding) => {
    setDraft(next);
    setDirty(true);
    setError('');
  };
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
