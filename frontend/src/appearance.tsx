import { useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import type { Branding } from './desktop-model';
import { Button, Logo } from './desktop-controls';
export function Appearance({
  brand,
  theme,
  setTheme,
  busy,
  total,
  companySelected = true,
  ready = true,
  catalogName = '',
  saveBrand,
}: {
  brand: Branding;
  theme: string;
  setTheme: (s: string) => void;
  busy: boolean;
  total?: number;
  companySelected?: boolean;
  ready?: boolean;
  catalogName?: string;
  saveBrand?: (b: Branding) => void | boolean | Promise<boolean>;
}) {
  const [draft, setDraft] = useState(brand),
    [dirty, setDirty] = useState(false),
    [error, setError] = useState('');
  const [loadingImage, setLoadingImage] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const reading = useRef<FileReader | null>(null);
  const generation = useRef(0);
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
  const change = (b: Branding) => {
    setDraft(b);
    setDirty(true);
    setError('');
  };
  return (
    <div className="settings">
      <div className="intro">
        <h1>Aparência</h1>
      </div>
      <div className="settings-layout">
        <div>
          <section className="setting-section">
            <h2>Interface</h2>
            <div>
              <div className="theme-choices">
                {[
                  ['light', 'Claro'],
                  ['dark', 'Escuro'],
                ].map(([id, name]) => (
                  <button
                    key={id}
                    className={
                      'theme-choice ' +
                      (id === 'dark' ? 'dark-sample ' : '') +
                      (theme === id ? 'selected' : '')
                    }
                    aria-pressed={theme === id}
                    disabled={busy}
                    onClick={() => setTheme(id)}
                  >
                    <span className="theme-sample">
                      <i />
                      <b />
                    </span>
                    <span>{name}</span>
                  </button>
                ))}
              </div>
            </div>
          </section>
          {companySelected ? (
            <section className="setting-section">
              <h2>Marca da empresa</h2>
              <div className="brand-form">
                <fieldset disabled={busy || !ready} className="brand-fields">
                  <div className="brand-logo-row">
                    <span>
                      <Logo brand={draft} />
                    </span>
                    <div>
                      <Button onClick={() => input.current?.click()}>
                        Escolher logo
                      </Button>
                      {draft.logo && (
                        <Button
                          kind="flat"
                          onClick={() => {
                            invalidateRead();
                            change({ ...draft, logo: '' });
                          }}
                        >
                          Remover
                        </Button>
                      )}
                    </div>
                    <input
                      ref={input}
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      hidden
                      onChange={(e) => {
                        const f = e.target.files?.[0];
                        invalidateRead();
                        if (!f) return;
                        if (
                          !['image/png', 'image/jpeg', 'image/webp'].includes(
                            f.type,
                          ) ||
                          f.size > 2 * 1024 * 1024
                        ) {
                          setError(
                            'Escolha uma imagem PNG, JPEG ou WebP de até 2 MB.',
                          );
                          return;
                        }
                        const reader = new FileReader();
                        const current = generation.current;
                        reading.current = reader;
                        setLoadingImage(true);
                        setDirty(true);
                        reader.onload = () => {
                          if (current !== generation.current) return;
                          reading.current = null;
                          setLoadingImage(false);
                          setDraft((previous) => ({
                            ...previous,
                            logo: String(reader.result),
                          }));
                          setDirty(true);
                          setError('');
                        };
                        reader.onerror = () => {
                          if (current === generation.current) {
                            reading.current = null;
                            setLoadingImage(false);
                            setError(
                              'Não foi possível ler a imagem. Escolha outra.',
                            );
                          }
                        };
                        reader.readAsDataURL(f);
                      }}
                    />
                  </div>
                  <div className="field">
                    <label htmlFor="brand-name">Nome visual</label>
                    <input
                      id="brand-name"
                      value={draft.name}
                      maxLength={48}
                      onChange={(e) =>
                        change({ ...draft, name: e.target.value })
                      }
                    />
                  </div>
                  <div style={{ marginTop: 23 }}>
                    <span
                      id="accent-label"
                      style={{ fontSize: 14, fontWeight: 500 }}
                    >
                      Cor de destaque
                    </span>
                    <div
                      className="swatches"
                      role="group"
                      aria-labelledby="accent-label"
                    >
                      {[
                        ['teal', 'Turquesa', '#0e7490'],
                        ['violet', 'Violeta', '#6d55b0'],
                        ['copper', 'Cobre', '#955635'],
                        ['blue', 'Azul', '#1d4ed8'],
                        ['green', 'Verde', '#166534'],
                        ['rose', 'Rosa', '#be185d'],
                        ['indigo', 'Índigo', '#4338ca'],
                        ['slate', 'Ardósia', '#475569'],
                      ].map(([id, name, color]) => (
                        <button
                          key={id}
                          id={'accent-' + id}
                          className={
                            'swatch ' + (draft.accent === id ? 'selected' : '')
                          }
                          aria-pressed={draft.accent === id}
                          style={{ '--color': color } as CSSProperties}
                          onClick={() => change({ ...draft, accent: id })}
                        >
                          <i aria-hidden="true" />
                          {name}
                        </button>
                      ))}
                    </div>
                  </div>
                </fieldset>
              </div>
            </section>
          ) : (
            <p className="outline-note">
              Selecione uma empresa para editar sua marca.
            </p>
          )}
          {companySelected && (
            <div className="settings-actions">
              <Button
                kind="primary"
                disabled={
                  !dirty || !saveBrand || busy || !ready || loadingImage
                }
                onClick={async () => {
                  invalidateRead();
                  if (await saveBrand?.(draft)) setDirty(false);
                }}
              >
                Salvar
              </Button>
              <Button
                kind="flat"
                disabled={!dirty || busy || !ready}
                onClick={() => {
                  invalidateRead();
                  setDraft(brand);
                  setDirty(false);
                  setError('');
                }}
              >
                Cancelar
              </Button>
              <Button
                kind="flat"
                disabled={busy || !ready}
                onClick={() => {
                  invalidateRead();
                  change({ name: '', logo: '', accent: 'teal' });
                }}
              >
                Restaurar padrão
              </Button>
            </div>
          )}
          {!saveBrand && companySelected && (
            <p className="outline-note">Carregando a marca da empresa.</p>
          )}
          {error && (
            <p role="alert" className="inline-error">
              {error}
            </p>
          )}
        </div>
        {companySelected && (
          <aside
            className="settings-preview"
            data-theme={theme}
            data-accent={brand.accent}
          >
            <h2>Marca salva</h2>
            <div className="preview-company">
              <Logo brand={{ ...brand, name: brand.name || catalogName }} />
              <span>{brand.name || catalogName}</span>
            </div>
            <div className="preview-item active">Dashboard</div>
            <div className="preview-item">Equipamentos</div>
            <div className="preview-item">Topologia</div>
            <div className="preview-reading">
              Sessões PPPoE<strong>{total ?? '—'}</strong>
            </div>
          </aside>
        )}
      </div>
    </div>
  );
}
