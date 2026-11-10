import { equipmentAPI as api } from './equipment-api';
import { equipmentType } from './equipment-layout';
import { latestSavedReading } from './equipment-reading-format';
import { driverOwners } from './equipment-role-options';
import { equipmentReadBlocked } from './equipment-query-policy';
import { formatDuration } from './network-format';
import { Button } from '../../shared/ui/Button';
import { PageHeader } from '../../shared/ui/PageHeader';

import type { Props } from './equipment-detail';

type HeaderProps = {
  p: Props;
  tab: string;
  isRadio: boolean;
  back: () => void;
  setRadioActions: React.Dispatch<React.SetStateAction<HTMLDivElement | null>>;
};
function headerActionLabel(p: Props) {
  if (p.job?.state === 'RUNNING') return 'Parar';
  return p.selected.fingerprint ? 'Atualizar' : 'Conectar';
}
function runHeaderAction(p: Props) {
  if (p.job?.state === 'RUNNING')
    void p.perform(
      () => api.cancel(p.profile, p.activation, p.job!.id),
      (r) => p.setJob(r.data?.job),
      false,
    );
  else
    void p.startOperation(
      p.selected.fingerprint ? 'query' : 'inspect',
      p.selected,
    );
}
function EquipmentHeaderActions({
  p,
  tab,
  isRadio,
  setRadioActions,
}: HeaderProps) {
  if (driverOwners[p.selected.role || '']) return null;
  if (isRadio || tab === 'Sessões' || tab === 'Serviços')
    return <div ref={setRadioActions} className="actions header-actions" />;
  return (
    <Button
      kind="primary"
      disabled={p.job?.state === 'RUNNING' ? p.locked : equipmentReadBlocked(p)}
      onClick={() => runHeaderAction(p)}
    >
      {headerActionLabel(p)}
    </Button>
  );
}
function readingAge(at: string) {
  return at
    ? `há ${formatDuration(Math.max(0, Date.now() - Date.parse(at)) / 1000)}`
    : 'Ainda sem leitura';
}
export function EquipmentSheetHeader(props: HeaderProps) {
  const { p, back } = props,
    device = p.selected,
    role = device.role || 'other';
  const saved = latestSavedReading(device);
  const age = readingAge(saved.at);
  return (
    <PageHeader
      title={device.name}
      meta={
        <>
          {device.name} · {equipmentType(role)}
          {device.block && ` · ${device.block}`} ·{' '}
          {saved.version || 'Não informado nesta leitura'} ·{' '}
          {saved.at ? `atualizado ${age}` : age}
        </>
      }
    >
      <Button onClick={back}>Voltar</Button>
      <EquipmentHeaderActions {...props} />
    </PageHeader>
  );
}
