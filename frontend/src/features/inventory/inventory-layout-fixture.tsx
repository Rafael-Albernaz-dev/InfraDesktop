import type { Device } from '../equipment/equipment-api';
import { RadioTransports } from '../radio/radio-contract';
import { prepareSecurityVisual } from '../access/security-layout-fixture';

export function prepareBase5Devices(
  devices: Device[],
  visualScene: string,
  time: number,
) {
  const groups: Record<string, string> = {
    'CCR-CENTRO': 'Região Centro',
    'AP-NORTE': 'Torre Norte',
    'CPE-0001': 'Torre Norte',
    'PTP-0001': 'Região Centro',
    'AP-SUL': 'Torre Sul',
  };
  const models: Record<string, string> = {
    'CCR-CENTRO': 'CCR1036',
    'AP-NORTE': 'RB912UAG',
    'CPE-0001': 'SXT Lite5',
    'PTP-0001': 'LHG 5',
    'AP-SUL': 'RB912UAG',
  };
  for (const d of devices) {
    d.block = groups[d.id] || '';
    if (d.id === 'EQP-0001') d.credential_id = '';
    if (visualScene.startsWith('equipamentos') && d.result)
      d.result.board = models[d.id] || '';
    if (d.id === 'CCR-CENTRO')
      d.pppoe = {
        online: 22,
        offline: 3,
        at: new Date(time - 12000).toISOString(),
      } as Device['pppoe'];
    if (d.id === 'AP-NORTE')
      d.radio_summary = {
        observed_at: d.result!.observed_at,
        version: d.result!.version,
        model: 'RB912UAG',
        cpu: 14,
        uptime: '30d1h',
        clients: 18,
      } as Device['radio_summary'];
  }
  for (const [name, role, address] of [
    ['SW-NORTE', 'switch', '192.0.2.60'],
    ['ENERGIA-NORTE', 'power-monitor', '192.0.2.70'],
  ]) {
    devices.push({
      id: name,
      name,
      role,
      block: 'Torre Norte',
      address,
      port: 22,
      transport: RadioTransports.ssh,
      credential_id: '',
      revision: 1,
      active: true,
      fingerprint: '',
      trust_origin: '',
    } as Device);
  }
}

export function createBase5List(
  devices: Device[],
  result: (data: Record<string, unknown>) => unknown,
) {
  return (
    search: string,
    cursor: number,
    inactive: boolean,
    role = '',
    block = '',
    empty = false,
    model = '',
    inactiveOnly = false,
  ) => {
    const filtered = devices.filter(
      (d) =>
        (inactiveOnly ? !d.active : inactive || d.active) &&
        `${d.name} ${d.address}`.toLowerCase().includes(search.toLowerCase()) &&
        (!role || d.role === role) &&
        (!block || d.block === block) &&
        (!empty || !d.block) &&
        (!model || (d.result?.board || d.radio_summary?.model) === model),
    );
    return result({
      devices: filtered.slice(cursor, cursor + 16),
      next_cursor: cursor + 16 < filtered.length ? cursor + 16 : 0,
      total_devices: devices.length,
      blocks: [...new Set(devices.map((d) => d.block).filter(Boolean))],
      models: [
        ...new Set(
          devices
            .map((d) => d.result?.board || d.radio_summary?.model)
            .filter(Boolean),
        ),
      ],
    });
  };
}

export function prepareBase5Scene(visualScene: string, doc: Document) {
  prepareSecurityVisual(visualScene, doc);
  if (
    ![
      'equipamentos-colunas',
      'equipamentos-selecao',
      'ficha-sw-norte-resumo',
    ].includes(visualScene)
  )
    return;
  const steps =
    visualScene === 'equipamentos-colunas'
      ? ['Colunas']
      : visualScene === 'equipamentos-selecao'
        ? ['Selecionar AP-NORTE', 'Selecionar CPE-0001']
        : ['SW-NORTE'];
  const observer = new MutationObserver(() => {
    const step = steps[0];
    const target =
      step === 'Colunas'
        ? [...doc.querySelectorAll('button')].find(
            (b) => b.textContent === step,
          )
        : step === 'SW-NORTE'
          ? doc.querySelector<HTMLElement>('[data-device-id="SW-NORTE"]')
          : doc.querySelector<HTMLInputElement>(`input[aria-label="${step}"]`);
    if (!target) return;
    steps.shift();
    target.click();
    if (visualScene === 'equipamentos-selecao' && !steps.length)
      target.closest('tr')?.focus();
    if (!steps.length) observer.disconnect();
  });
  observer.observe(doc.documentElement, {
    childList: true,
    subtree: true,
  });
}
