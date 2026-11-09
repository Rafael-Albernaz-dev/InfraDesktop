export function layoutVerdict({ results, pending, sceneIds }) {
  const problems = [];
  const declared = new Set();
  for (const entry of pending) {
    if (!sceneIds.includes(entry.id)) problems.push(`Cena pendente inexistente: ${entry.id}`);
    if (declared.has(entry.id)) problems.push(`Cena pendente repetida: ${entry.id}`);
    if (typeof entry.owner !== 'string' || !entry.owner.trim() || typeof entry.reason !== 'string' || !entry.reason.trim())
      problems.push(`Cena pendente sem dono ou motivo: ${entry.id}`);
    declared.add(entry.id);
  }
  if (!results.length) problems.push('Nenhum resultado de layout');
  for (const result of results) {
    const scene = result.id.replace(/-(claro|escuro)$/, '');
    const parked = declared.has(scene) && result.reason === 'Cena pendente' && !result.errors?.length;
    if (!result.pass && !parked) problems.push(`Falha de layout: ${result.id}`);
  }
  return { exitCode: problems.length ? 1 : 0, problems };
}
