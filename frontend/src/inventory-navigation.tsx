import { useCallback, useRef, useState } from 'react';

export function useInventoryNavigation(
  profileId: string,
  activationId: string,
) {
  const [inventoryTarget, setInventoryTarget] = useState({ key: '', id: '' });
  const [inventoryDetail, setInventoryDetail] = useState(false);
  const [inventoryBlocked, setInventoryBlocked] = useState(false);
  const detailBack = useRef<(() => void) | undefined>(undefined);
  const registerDetailBack = useCallback((back?: () => void) => {
    detailBack.current = back;
  }, []);
  const [inventoryState, setInventoryState] = useState({ key: '', total: 0 });
  const contextPair = [profileId, activationId].join('|');
  const reportInventory = useCallback((key: string, total: number) => {
    setInventoryState((old) =>
      old.key === key && old.total === total ? old : { key, total },
    );
  }, []);
  const selectInventory = useCallback(
    (id: string) => setInventoryTarget({ key: contextPair, id }),
    [contextPair],
  );
  return {
    contextPair,
    inventoryTarget,
    inventoryDetail,
    inventoryBlocked,
    setInventoryBlocked,
    detailBack,
    registerDetailBack,
    setInventoryDetail,
    inventoryState,
    reportInventory,
    selectInventory,
  };
}
