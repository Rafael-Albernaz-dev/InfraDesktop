import { useState } from 'react';
import { Equipment as EquipmentView } from './equipment';
import type { EquipmentProps } from './equipment';

export function Equipment(props: EquipmentProps) {
  const [state, setState] = useState({
    prop: props.addPage,
    adding: Boolean(props.addPage),
  });
  if (state.prop !== props.addPage)
    setState({ prop: props.addPage, adding: Boolean(props.addPage) });
  const setAdding = (adding: boolean) =>
    setState({ prop: props.addPage, adding });
  const adding = state.adding;
  return (
    <EquipmentView
      {...props}
      addPage={adding}
      onAdd={() => {
        setAdding(true);
        props.onAdd?.();
      }}
      onAddBack={() => {
        setAdding(false);
        props.onAddBack?.();
      }}
    />
  );
}
