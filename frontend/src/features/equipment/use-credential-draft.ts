import { useEffect, useRef, useState } from 'react';
import type {
  Credential,
  CredentialInput,
  EquipmentResult,
} from './equipment-api';
import { useDraftProtection, useFieldErrors } from '../access/equipment-draft';
import { id } from './equipment-reading-format';
import {
  fieldForEquipmentCode,
  fieldMessages,
  validEquipmentSecret,
  validEquipmentText,
} from './equipment-validation';
type FormOptions = {
  credential?: Credential;
  disabled: boolean;
  save: (d: CredentialInput) => Promise<EquipmentResult | undefined>;
};
function useInitialCredentialDraft(credential?: Credential) {
  const [draft, setDraft] = useState<CredentialInput>(() => ({
    id: credential?.id || id(),
    operation_id: id(),
    revision: credential?.revision || 0,
    alias: credential?.alias || '',
    user: credential?.user || '',
    secret: '',
    affected_links: credential?.links || 0,
  }));
  const [sent, setSent] = useState(false);
  return { draft, setDraft, sent, setSent };
}
function credentialErrors(draft: CredentialInput, existing: boolean) {
  const errors: Record<string, string> = {};
  if (!validEquipmentText(draft.alias)) errors.alias = fieldMessages.alias;
  if (!validEquipmentText(draft.user)) errors.user = fieldMessages.user;
  if ((!existing || draft.secret) && !validEquipmentSecret(draft.secret))
    errors.secret = fieldMessages.secret;
  return errors;
}
type Submission = ReturnType<typeof useInitialCredentialDraft> &
  FormOptions & {
    validation: ReturnType<typeof useFieldErrors>;
    submitting: React.MutableRefObject<boolean>;
    alive: React.MutableRefObject<boolean>;
    setConfirming: (v: boolean) => void;
  };
function sendCredential(c: Submission) {
  c.submitting.current = true;
  const request = {
    ...c.draft,
    alias: c.draft.alias.trim(),
    user: c.draft.user.trim(),
  };
  c.setDraft({ ...c.draft, secret: '' });
  c.setSent(Boolean(c.draft.secret));
  void (async () => {
    try {
      const result = await c.save(request);
      if (c.alive.current && result && fieldForEquipmentCode[result.code]) {
        const field = fieldForEquipmentCode[result.code];
        c.validation.setErrors({ [field]: fieldMessages[field] });
      }
    } catch {
      // The perform owner reports transport failures; the submitted secret is discarded below.
    } finally {
      request.secret = '';
      c.submitting.current = false;
    }
  })();
}
function requestCredentialSave(c: Submission, confirmed: boolean) {
  if (c.submitting.current || c.disabled) return;
  const errors = credentialErrors(c.draft, Boolean(c.credential));
  c.validation.setErrors(errors);
  if (Object.keys(errors).length) return;
  if (!confirmed && c.draft.secret && (c.credential?.links || 0) > 0) {
    c.setConfirming(true);
    return;
  }
  c.setConfirming(false);
  sendCredential(c);
}
export function useCredentialDraft(p: FormOptions) {
  const state = useInitialCredentialDraft(p.credential),
    validation = useFieldErrors(p.disabled);
  const [confirming, setConfirming] = useState(false),
    submitting = useRef(false),
    alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const cancel = useDraftProtection(() => ({
    dirty:
      state.draft.alias !== (p.credential?.alias || '') ||
      state.draft.user !== (p.credential?.user || '') ||
      state.draft.secret !== '',
    blocked: submitting.current,
  }));
  const context: Submission = {
    ...state,
    ...p,
    validation,
    submitting,
    alive,
    setConfirming,
  };
  return {
    ...state,
    validation,
    cancel,
    confirming,
    setConfirming,
    submit: (confirmed: boolean) => requestCredentialSave(context, confirmed),
  };
}
