import { useContext, useEffect, useState } from 'react';
import { DetailTitle } from './ui/DetailTitle';
import { Button } from './ui/Button';
import { Tabs } from './ui/Tabs';
import { Field } from './ui/Field';
import type { useEquipmentState } from './equipment-state';
import type { ImportRow } from './presentation';
import { RadioRoles } from './radio-contract';
import { AddEquipmentFields } from './add-equipment-fields';
import { useAddNavigationGuard } from './add-navigation-guard';
import { useImportState } from './add-import-state';
import { ImportFile, ImportSteps } from './add-import-file';
import { ImportPreview } from './add-import-preview';
import { useAddEnrollment } from './add-enrollment';
import { useFieldErrors } from './equipment-draft';
import {
  fieldMessages,
  validEquipmentAddress,
  validEquipmentText,
  validEquipmentSecret,
} from './equipment-validation';

const emptyRow = (): ImportRow => ({
  line: 1,
  name: '',
  address: '',
  port: '22',
  role: RadioRoles.ap,
  block: '',
  credential: '',
});
export function AddEquipment(p: {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  back: () => void;
  locked: boolean;
}) {
  const [tab, setTab] = useState<'one' | 'file'>('one');
  const [row, setRow] = useState(emptyRow);
  const [singleError, setSingleError] = useState('');
  const [singleDirty, setSingleDirty] = useState(false);
  const [credentialChosen, setCredentialChosen] = useState(false);
  const [inlineUser, setInlineUser] = useState('');
  const [inlineSecret, setInlineSecret] = useState('');
  const {
    ref: validationRef,
    setErrors: setValidationErrors,
    attrs: validationAttrs,
    message: validationMessage,
  } = useFieldErrors(p.locked);
  const s = useImportState(p.profile, p.activation);
  const enrollment = useAddEnrollment({
    profile: p.profile,
    activation: p.activation,
    perform: p.s.perform,
    reloadCredentials: p.s.reloadCredentials,
    continueEnrollment: p.s.continueEnrollment,
    setErrors: setValidationErrors,
    saved: (another) => {
      setRow(emptyRow());
      setSingleDirty(false);
      setCredentialChosen(false);
      setInlineUser('');
      setInlineSecret('');
      if (!another) p.back();
    },
  });
  const setTitle = useContext(DetailTitle);

  useEffect(() => {
    setTitle('Adicionar equipamento');
    return () => setTitle('');
  }, [setTitle]);
  const shownRow = credentialChosen
    ? row
    : { ...row, credential: p.s.credentialDefaults[row.role] || '' };
  const guard = useAddNavigationGuard(
    tab === 'one'
      ? singleDirty || enrollment.uncertain
      : !!s.items.length && !s.receipt,
    s.busy || p.s.busy,
    !p.s.credentialForm,
  );
  const back = () => guard.attempt(p.back);
  const change = (patch: Partial<ImportRow>) => {
    if (
      patch.role ||
      ('credential' in patch && patch.credential !== '__inline__')
    ) {
      setInlineUser('');
      setInlineSecret('');
    }
    if (patch.role && patch.role !== row.role) setCredentialChosen(false);
    else if ('credential' in patch) setCredentialChosen(true);
    setRow({ ...shownRow, ...patch });
    setSingleDirty(true);
    enrollment.changed();
    setSingleError('');
  };
  const save = (another: boolean) => {
    if (
      p.locked ||
      p.s.busy ||
      enrollment.checking ||
      (shownRow.credential === '__inline__' && !p.s.vault)
    )
      return;
    const port = Number(row.port);
    const errors: Record<string, string> = {};
    if (!validEquipmentText(row.name)) errors.name = fieldMessages.name;
    if (!validEquipmentAddress(row.address))
      errors.address = fieldMessages.address;
    if (!Number.isInteger(port) || port < 1 || port > 65535)
      errors.port = fieldMessages.port;
    if (shownRow.credential === '__inline__') {
      if (!validEquipmentText(inlineUser)) errors.user = fieldMessages.user;
      if (!validEquipmentSecret(inlineSecret))
        errors.secret = fieldMessages.secret;
    }
    setValidationErrors(errors);
    if (Object.keys(errors).length || !row.role) {
      if (!row.role) setSingleError('Selecione o papel do equipamento.');
      return;
    }
    const secret = inlineSecret;
    setInlineSecret('');
    void enrollment.save(shownRow, another, inlineUser, secret);
  };
  return (
    <section className="add-equipment">
      <div className="head">
        <h1 className="grow">Adicionar equipamento</h1>
        <Button onClick={back}>Voltar</Button>
      </div>
      <Tabs
        label="Adicionar equipamento"
        items={
          [
            ['one', 'Um equipamento'],
            ['file', 'Importar arquivo'],
          ] as const
        }
        value={tab}
        onChange={(next) =>
          next === tab
            ? undefined
            : guard.attempt(() => {
                if (next === tab) return;
                if (tab === 'file') s.reset();
                else {
                  setRow(emptyRow());
                  setSingleDirty(false);
                  setCredentialChosen(false);
                  setInlineUser('');
                  setInlineSecret('');
                }
                setTab(next);
              })
        }
      />
      {tab === 'one' && enrollment.uncertain ? (
        <div role="status" className="inventory-notice">
          <p>
            Cadastro ainda não confirmado. Confira o resultado antes de fechar.
          </p>
          <Button
            disabled={enrollment.checking || p.locked}
            onClick={() => void enrollment.reconcile()}
          >
            Conferir cadastro
          </Button>
        </div>
      ) : tab === 'one' ? (
        <form
          noValidate
          ref={validationRef}
          onSubmit={(event) => {
            event.preventDefault();
            save(false);
          }}
        >
          {enrollment.retry && (
            <p className="inventory-notice">
              Para tentar com o mesmo cadastro, confira os dados
              {shownRow.credential === '__inline__'
                ? ' e digite a senha novamente.'
                : '.'}
            </p>
          )}
          {singleError && (
            <p className="field-error" role="alert">
              {singleError}
            </p>
          )}
          <AddEquipmentFields
            row={shownRow}
            credentials={p.s.credentials}
            blocks={p.s.blocks}
            credentialById
            disabled={p.locked}
            addCredential={() => p.s.setCredentialForm('new')}
            validation={{ attrs: validationAttrs, message: validationMessage }}
            change={change}
          />
          {shownRow.credential === '__inline__' && (
            <div className="add-form-grid">
              <Field
                layout="form"
                label="Usuário"
                aria-label="Usuário"
                {...validationAttrs('user')}
                value={inlineUser}
                disabled={p.locked}
                maxLength={80}
                onChange={(event) => {
                  setInlineUser(event.target.value);
                  setSingleDirty(true);
                  enrollment.changed();
                }}
              >
                {validationMessage('user')}
              </Field>
              <Field
                layout="form"
                label="Senha"
                aria-label="Senha"
                type="password"
                {...validationAttrs('secret')}
                value={inlineSecret}
                maxLength={1024}
                autoComplete="new-password"
                disabled={p.locked || !p.s.vault}
                onChange={(event) => {
                  setInlineSecret(event.target.value);
                  setSingleDirty(true);
                  enrollment.changed();
                }}
              >
                {validationMessage('secret')}
              </Field>
            </div>
          )}
          <div className="actions">
            <Button onClick={back}>Cancelar</Button>
            <Button
              disabled={
                p.locked || (shownRow.credential === '__inline__' && !p.s.vault)
              }
              onClick={() => save(true)}
            >
              Salvar e adicionar outro
            </Button>
            <Button
              kind="primary"
              type="submit"
              disabled={
                p.locked || (shownRow.credential === '__inline__' && !p.s.vault)
              }
            >
              Salvar
            </Button>
          </div>
        </form>
      ) : s.receipt ? (
        <>
          <ImportSteps step={3} />
          <section className="import-card">
            <h2>Importação concluída</h2>
            <p>
              <b>{s.receipt.created} equipamentos cadastrados</b>,{' '}
              {s.receipt.ignored} linhas ignoradas. Nenhuma consulta foi feita.
            </p>
            <div className="actions">
              <Button kind="primary" onClick={p.back}>
                Ver equipamentos
              </Button>
              <Button onClick={s.reset}>Importar outro arquivo</Button>
            </div>
          </section>
        </>
      ) : s.items.length ? (
        <ImportPreview
          state={s}
          credentials={p.s.credentials}
          cancel={back}
          replace={() => guard.attempt(s.reset)}
        />
      ) : (
        <ImportFile state={s} credentials={p.s.credentials} />
      )}
      {s.error && (
        <p className="field-error" role="alert">
          {s.error}
        </p>
      )}
      {guard.dialog}
    </section>
  );
}
