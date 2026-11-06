import { useState } from 'react';
import type { Device } from './equipment-api';
import type { useEquipmentState } from './equipment-state';
import { InventoryToolbar } from './inventory-toolbar';
import { InventoryTable } from './inventory-table';
import { useColumnPreferences } from './inventory-preferences';
import { InventoryBulkDialog } from './inventory-bulk-dialog';
import { Button } from './ui/Button';
import { Pager } from './ui/Pager';

type Props = ReturnType<typeof useEquipmentState> & {
  profile: string;
  activation: string;
  connected: boolean;
  primaryInHeader: boolean;
  locked: boolean;
  onAdd?: () => void;
};
export function EquipmentInventoryPane(p: Props) {
  const preferences = useColumnPreferences();
  const [bulk, setBulk] = useState<'block' | 'credential'>();
  const [selectedRows, setSelectedRows] = useState<Record<string, Device>>({});
  const state = { ...p, selectedRows, setSelectedRows };
  const selected = Object.values(selectedRows);
  return (
    <section hidden={p.detailOpen}>
      <h1>Equipamentos</h1>
      <InventoryToolbar
        p={state}
        locked={p.locked}
        columns={preferences.visible}
        saveColumns={(v) => void preferences.save(v)}
        busy={preferences.busy}
        onAdd={p.onAdd}
      />
      {preferences.error && <p role="alert">{preferences.error}</p>}
      {selected.length > 0 && (
        <div className="inventory-selection" role="status">
          <b>
            {selected.length} selecionado{selected.length === 1 ? '' : 's'}
          </b>
          <Button disabled={p.locked} onClick={() => setBulk('block')}>
            Definir bloco
          </Button>
          <Button disabled={p.locked} onClick={() => setBulk('credential')}>
            Definir credencial
          </Button>
          <span className="grow" />
          <Button onClick={() => setSelectedRows({})}>Limpar seleção</Button>
        </div>
      )}
      {!p.ready && !p.listError && (
        <p role="status">Carregando equipamentos…</p>
      )}
      <InventoryTable p={state} visible={preferences.visible} />
      <Pager
        page={p.cursor ? 2 : 1}
        total={p.next ? undefined : p.cursor ? 2 : 1}
        hasNext={Boolean(p.next)}
        first={() => p.setCursor(0)}
        next={() => p.setCursor(p.next)}
      />
      {bulk && selected.length > 0 && (
        <InventoryBulkDialog
          mode={bulk}
          profile={p.profile}
          activation={p.activation}
          selected={selected}
          credentials={p.credentials}
          blocks={p.blocks}
          close={() => setBulk(undefined)}
          applied={() => {
            setBulk(undefined);
            setSelectedRows({});
            p.reload((n) => n + 1);
          }}
        />
      )}
    </section>
  );
}
