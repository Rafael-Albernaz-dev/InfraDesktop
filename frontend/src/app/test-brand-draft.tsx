// Isolated test UI for retained draft behavior; never imported by the application.
import type { Branding } from '../features/access/desktop-model';
import { useBrandDraft } from '../features/access/use-brand-draft';
import { Button } from '../shared/ui/Button';
export function BrandDraftHarness(p: {
  brand: Branding;
  busy: boolean;
  ready?: boolean;
  theme: string;
  setTheme: (theme: string) => void;
  companySelected?: boolean;
  saveBrand?: (b: Branding) => void | boolean | Promise<boolean>;
}) {
  const {
    draft,
    dirty,
    error,
    input: inputRef,
    change,
    readFile,
    cancel,
    reset,
    remove,
    save,
    canSave,
  } = useBrandDraft(p.brand, p.busy, p.ready ?? true, p.saveBrand);
  return (
    <>
      <p>{p.brand.name}</p>
      <fieldset disabled={p.busy || p.ready === false}>
        <label>
          Nome visual
          <input
            value={draft.name}
            onChange={(e) => change({ ...draft, name: e.target.value })}
          />
        </label>
        <input
          ref={inputRef}
          type="file"
          onChange={(e) => readFile(e.target.files?.[0])}
        />
        {draft.logo && <img src={draft.logo} alt="Rascunho" />}
        {(['teal', 'violet'] as const).map((accent) => (
          <Button
            key={accent}
            aria-pressed={draft.accent === accent}
            onClick={() => change({ ...draft, accent })}
          >
            {accent === 'teal' ? 'Turquesa' : 'Violeta'}
          </Button>
        ))}
        <Button disabled={!canSave} onClick={() => void save()}>
          Salvar
        </Button>
        <Button disabled={!dirty} onClick={cancel}>
          Cancelar
        </Button>
        <Button onClick={reset}>Restaurar padrão</Button>
        <Button onClick={remove}>Remover</Button>
        {error && <p role="alert">{error}</p>}
      </fieldset>
    </>
  );
}
