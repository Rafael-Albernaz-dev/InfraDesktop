// Fachada única do adaptador Wails: as telas importam daqui e os testes mocam este módulo.
// Cada capacidade vive no próprio arquivo desta pasta, o único lugar que importa os bindings gerados.
export * from './access';
export * from './equipment';
export * from './inventory';
export * from './pppoe';
export * from './pppoe-types';
export * from './radio';
