import type { AddEquipmentProps } from './add-equipment';
import { AddEquipmentFields } from './add-equipment-fields';
import { Button } from './ui/Button';
import { Field } from './ui/Field';
import type { AddEquipmentModel } from './use-add-equipment-page';
type AddBodyProps = AddEquipmentProps & { model: AddEquipmentModel };

function SingleEnrollmentNotices(p: AddBodyProps) {
  const { enrollment, shownRow, singleError } = p.model;
  return (
    <>
      {enrollment.retry && (
        <p className="inventory-notice">
          Para tentar com o mesmo cadastro, confira os dados
          {shownRow.credential === '__inline__'
            ? ' e digite a senha novamente.'
            : '.'}
        </p>
      )}
      {enrollment.needsSecret && (
        <p role="status">Digite a senha novamente para tentar salvar.</p>
      )}
      {singleError && (
        <p className="field-error" role="alert">
          {singleError}
        </p>
      )}
    </>
  );
}
function SingleInlineFields(p: AddBodyProps) {
  const {
    shownRow,
    validationAttrs,
    validationMessage,
    inlineUser,
    inlineSecret,
    setInlineUser,
    setInlineSecret,
    setSingleDirty,
    enrollment,
  } = p.model;
  return (
    <>
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
    </>
  );
}
function SingleFormActions(p: AddBodyProps) {
  const { back, shownRow, save } = p.model;
  return (
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
  );
}
export function SingleEquipmentForm(p: AddBodyProps) {
  const {
    validationRef,
    save,
    shownRow,
    validationAttrs,
    validationMessage,
    change,
  } = p.model;
  return (
    <form
      noValidate
      ref={validationRef}
      onSubmit={(event) => {
        event.preventDefault();
        save(false);
      }}
    >
      <SingleEnrollmentNotices {...p} />
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
      <SingleInlineFields {...p} />
      <SingleFormActions {...p} />
    </form>
  );
}
