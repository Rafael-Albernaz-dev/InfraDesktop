import { ReadingOperation,queryFailureOwnsDetail } from './equipment-query-failure';
export function EquipmentQueryFailure({
  state,
  tab,
  isRadio,
}: {
  state: ReadingOperation;
  tab: string;
  isRadio: boolean;
}) {
  if (!queryFailureOwnsDetail(state) || (!isRadio && tab === 'Resumo'))
    return null;
  return <p role="alert">{state.error}</p>;
}
