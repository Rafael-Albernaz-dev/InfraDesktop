export function tickFieldReason(raw: string) {
  const lower = raw.toLowerCase();
  const summaries = [
    ['tick-frame', 'O equipamento respondeu com um ciclo incompleto.'],
    ['tick-cpu', 'A carga de CPU não foi informada neste ciclo.'],
    [
      'tick-metric:',
      'Uma métrica não foi informada neste ciclo; as demais foram preservadas.',
    ],
    // Historical parse records remain readable.
    ['parse', 'O equipamento respondeu com campos incompletos neste ciclo.'],
  ];
  const found = summaries.find(([key]) => lower.includes(key));
  return found
    ? {
        summary: found[1],
        action: 'Aguarde o próximo ciclo ou repita a leitura manual.',
        technical: raw,
      }
    : undefined;
}
