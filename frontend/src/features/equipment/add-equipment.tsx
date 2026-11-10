import { AddEquipmentBody } from './add-equipment-body';
import type { useEquipmentState } from './equipment-state';
import { Button } from '../../shared/ui/Button';
import { Tabs } from '../../shared/ui/Tabs';
import { useAddEquipmentPage } from './use-add-equipment-page';

export type AddEquipmentProps = {
  s: ReturnType<typeof useEquipmentState>;
  profile: string;
  activation: string;
  back: () => void;
  locked: boolean;
};

export function AddEquipment(p: AddEquipmentProps) {
  const model = useAddEquipmentPage(p),
    { tab, s, guard, back, changeTab } = model;
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
        onChange={changeTab}
      />
      <AddEquipmentBody {...p} model={model} />
      {s.error && (
        <p className="field-error" role="alert">
          {s.error}
        </p>
      )}
      {guard.dialog}
    </section>
  );
}
