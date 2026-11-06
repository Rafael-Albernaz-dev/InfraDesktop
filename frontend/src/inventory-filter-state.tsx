import { useState } from 'react';

export function useInventoryFilterState() {
  const [roleFilter, setRoleFilter] = useState('');
  const [blockFilter, setBlockFilter] = useState('');
  const [modelFilter, setModelFilter] = useState('');
  const [blocks, setBlocks] = useState<string[]>([]);
  const [models, setModels] = useState<string[]>([]);
  return {
    roleFilter,
    setRoleFilter,
    blockFilter,
    setBlockFilter,
    modelFilter,
    setModelFilter,
    blocks,
    setBlocks,
    models,
    setModels,
  };
}
