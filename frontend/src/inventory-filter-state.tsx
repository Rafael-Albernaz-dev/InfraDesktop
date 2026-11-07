import { useState } from 'react';

export function useInventoryFilterState() {
  const [stateFilter, setStateFilter] = useState('');
  const [pulseNetwork, setPulseNetwork] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [blockFilter, setBlockFilter] = useState('');
  const [modelFilter, setModelFilter] = useState('');
  const [blocks, setBlocks] = useState<string[]>([]);
  const [models, setModels] = useState<string[]>([]);
  return {
    stateFilter,
    setStateFilter,
    pulseNetwork,
    setPulseNetwork,
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
