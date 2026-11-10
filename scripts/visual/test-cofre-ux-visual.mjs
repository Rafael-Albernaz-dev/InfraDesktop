// Historical renderer retired: current L-BASE visual oracle only; native/behavior proof stays separate.
process.argv.push('--scene', 'equipamentos');
process.argv.push('--scene', 'credenciais');
console.warn('Oráculo visual histórico retirado: somente cenas L-BASE; validações comportamentais/nativas históricas são NOT RUN.');
await import('./test-layout.mjs');
