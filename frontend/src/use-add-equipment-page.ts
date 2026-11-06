import { useContext, useEffect, useState } from 'react';
import { useAddEnrollment } from './add-enrollment';
import { useImportState } from './add-import-state';
import { useAddNavigationGuard } from './add-navigation-guard';
import { useFieldErrors } from './equipment-draft';
import {
  fieldMessages,
  validEquipmentAddress,
  validEquipmentSecret,
  validEquipmentText,
} from './equipment-validation';
import type { ImportRow } from './presentation';
import { RadioRoles } from './radio-contract';
import { DetailTitle } from './ui/DetailTitle';

import type { AddEquipmentProps } from './add-equipment';
const emptyRow = (): ImportRow => ({
  line: 1,
  name: '',
  address: '',
  port: '22',
  role: RadioRoles.ap,
  block: '',
  credential: '',
});

function useSingleDraft(locked: boolean) {
  const [row, setRow] = useState(emptyRow),
    [singleError, setSingleError] = useState(''),
    [singleDirty, setSingleDirty] = useState(false),
    [credentialChosen, setCredentialChosen] = useState(false),
    [inlineUser, setInlineUser] = useState(''),
    [inlineSecret, setInlineSecret] = useState('');
  const validation = useFieldErrors(locked);
  const reset = () => {
    setRow(emptyRow());
    setSingleDirty(false);
    setCredentialChosen(false);
    setInlineUser('');
    setInlineSecret('');
  };
  return {
    row,
    setRow,
    singleError,
    setSingleError,
    singleDirty,
    setSingleDirty,
    credentialChosen,
    setCredentialChosen,
    inlineUser,
    setInlineUser,
    inlineSecret,
    setInlineSecret,
    validation,
    reset,
  };
}
type SingleDraft = ReturnType<typeof useSingleDraft>;
function changeDraft(
  d: SingleDraft,
  shownRow: ImportRow,
  enrollment: ReturnType<typeof useAddEnrollment>,
  patch: Partial<ImportRow>,
) {
  if (
    patch.role ||
    ('credential' in patch && patch.credential !== '__inline__')
  ) {
    d.setInlineUser('');
    d.setInlineSecret('');
  }
  if (patch.role && patch.role !== d.row.role) d.setCredentialChosen(false);
  else if ('credential' in patch) d.setCredentialChosen(true);
  d.setRow({ ...shownRow, ...patch });
  d.setSingleDirty(true);
  enrollment.changed();
  d.setSingleError('');
}
function draftErrors(d: SingleDraft, shownRow: ImportRow) {
  const row = d.row,
    port = Number(row.port),
    errors: Record<string, string> = {};
  if (!validEquipmentText(row.name)) errors.name = fieldMessages.name;
  if (!validEquipmentAddress(row.address))
    errors.address = fieldMessages.address;
  if (!Number.isInteger(port) || port < 1 || port > 65535)
    errors.port = fieldMessages.port;
  if (shownRow.credential === '__inline__') {
    if (!validEquipmentText(d.inlineUser)) errors.user = fieldMessages.user;
    if (!validEquipmentSecret(d.inlineSecret))
      errors.secret = fieldMessages.secret;
  }
  return errors;
}
function saveSingle(
  p: AddEquipmentProps,
  s: ReturnType<typeof useSingleEquipment>,
  another: boolean,
) {
  const { enrollment, shownRow } = s;
  if (
    p.locked ||
    p.s.busy ||
    enrollment.checking ||
    (shownRow.credential === '__inline__' && !p.s.vault)
  )
    return;
  const errors = draftErrors(s, shownRow);
  s.validation.setErrors(errors);
  if (Object.keys(errors).length || !s.row.role) {
    if (!s.row.role) s.setSingleError('Selecione o papel do equipamento.');
    return;
  }
  const secret = s.inlineSecret;
  s.setInlineSecret('');
  void enrollment.save(shownRow, another, s.inlineUser, secret);
}
function useSingleEquipment(p: AddEquipmentProps) {
  const d = useSingleDraft(p.locked);
  const enrollment = useAddEnrollment({
    profile: p.profile,
    activation: p.activation,
    perform: p.s.perform,
    reloadCredentials: p.s.reloadCredentials,
    setErrors: d.validation.setErrors,
    saved: (another) => {
      d.reset();
      if (!another) p.back();
    },
  });
  const shownRow = d.credentialChosen
    ? d.row
    : { ...d.row, credential: p.s.credentialDefaults[d.row.role] || '' };
  return { ...d, enrollment, shownRow };
}
function useAddTitle() {
  const setTitle = useContext(DetailTitle);
  useEffect(() => {
    setTitle('Adicionar equipamento');
    return () => setTitle('');
  }, [setTitle]);
}
export function useAddEquipmentPage(p: AddEquipmentProps) {
  const [tab, setTab] = useState<'one' | 'file'>('one'),
    single = useSingleEquipment(p),
    s = useImportState(p.profile, p.activation);
  const { enrollment, singleDirty } = single;
  useAddTitle();
  const guard = useAddNavigationGuard(
    tab === 'one'
      ? singleDirty || enrollment.uncertain
      : !!s.items.length && !s.receipt,
    s.busy || p.s.busy,
    !p.s.credentialForm,
  );
  const back = () => guard.attempt(p.back);
  const changeTab = (next: 'one' | 'file') => {
    if (next === tab) return;
    guard.attempt(() => {
      if (next === tab) return;
      if (tab === 'file') s.reset();
      else single.reset();
      setTab(next);
    });
  };
  return {
    ...single,
    tab,
    s,
    guard,
    back,
    changeTab,
    validationRef: single.validation.ref,
    validationAttrs: single.validation.attrs,
    validationMessage: single.validation.message,
    change: (patch: Partial<ImportRow>) =>
      changeDraft(single, single.shownRow, enrollment, patch),
    save: (another: boolean) => saveSingle(p, single, another),
  };
}
export type AddEquipmentModel = ReturnType<typeof useAddEquipmentPage>;
