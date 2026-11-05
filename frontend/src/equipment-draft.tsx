import {
  createContext,
  useContext,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

export type DraftState = { dirty: boolean; blocked: boolean };

export const DraftProtection = createContext<{
  register: (read: () => DraftState) => void;
  cancel: () => void;
  attempt: (action: () => void) => void;
} | null>(null);

export function useDraftProtection(read: () => DraftState) {
  const protection = useContext(DraftProtection);
  useLayoutEffect(() => {
    protection?.register(read);
  });
  return protection?.cancel;
}

export function useFieldErrors(disabled: boolean) {
  const prefix = useId();
  const ref = useRef<HTMLFormElement>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  useLayoutEffect(() => {
    const field = Object.keys(errors)[0];
    if (field) {
      const control = ref.current?.querySelector<HTMLElement>(
        `[name="${field}"]:not(:disabled)`,
      );
      control?.closest('details')?.setAttribute('open', '');
      control?.focus();
    }
  }, [errors, disabled]);
  const attrs = (name: string) => ({
    name,
    'aria-invalid': errors[name] ? true : undefined,
    'aria-describedby': errors[name] ? `${prefix}-${name}` : undefined,
  });
  const message = (name: string) =>
    errors[name] ? (
      <span id={`${prefix}-${name}`} className="field-error">
        {errors[name]}
      </span>
    ) : null;
  return { ref, errors, setErrors, attrs, message };
}
