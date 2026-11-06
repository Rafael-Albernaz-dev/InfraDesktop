function importFixture(scene: string) {
  const values = [
    [
      'AP-LESTE-1',
      '192.0.2.31',
      'AP',
      'Torre Leste',
      'leitura-radio',
      'ready',
      '',
    ],
    [
      'AP-LESTE-2',
      '192.0.2.32',
      'AP',
      'Torre Leste',
      'leitura-radio',
      'ready',
      '',
    ],
    [
      'ST-LESTE',
      '192.0.2.33',
      'ST',
      'Torre Leste',
      'leitura-radio',
      'ready',
      '',
    ],
    ['SW-LESTE', '192.0.2.34', 'Switch', 'Torre Leste', '', 'ready', ''],
    [
      'ENERGIA-LESTE',
      '192.0.2.35',
      'Monitor de energia',
      'Torre Leste',
      '',
      'ready',
      '',
    ],
    [
      'PTP-LESTE',
      '192.0.2.36',
      'PTP',
      'Região Sul',
      'leitura-ptp',
      'ready',
      '',
    ],
    [
      'CPE-0100',
      '192.0.2.140',
      'CPE',
      'Região Sul',
      'leitura-radio',
      'ready',
      '',
    ],
    [
      'AP-NORTE-2',
      '192.0.2.20',
      'AP',
      'Torre Norte',
      'leitura-radio',
      'duplicate',
      'IP já cadastrado em AP-NORTE',
    ],
    [
      'AP-OESTE',
      '192.0.2.40',
      'AP',
      'Região Sul',
      'leitura-radio',
      'conflict',
      'Mesmo IP da linha 11',
    ],
    [
      'AP-OESTE-B',
      '192.0.2.40',
      'AP',
      'Região Sul',
      'leitura-radio',
      'conflict',
      'Mesmo IP da linha 10',
    ],
    [
      'RADIO-X',
      '192.0.2.999',
      'Rádio',
      '',
      '',
      'pending',
      'IP inválido e papel desconhecido',
    ],
    [
      'CPE-0101',
      '192.0.2.141',
      'CPE',
      'Região Sul',
      'credencial-antiga',
      'pending',
      'Credencial não encontrada no cofre',
    ],
  ];
  return values.map(
    ([name, address, role, block, credential, status, reason], i) => ({
      row: {
        line: i + 2,
        name,
        address,
        port: '22',
        role,
        block,
        credential,
        ignored:
          (scene === 'importar-fim' && i >= 7) ||
          (scene === 'importar-corrigir' && (i === 7 || i === 9)),
      },
      status:
        (scene === 'importar-fim' && i >= 7) ||
        (scene === 'importar-corrigir' && (i === 7 || i === 9))
          ? 'ignored'
          : status,
      reason: scene === 'importar-fim' && i >= 7 ? '' : reason,
    }),
  );
}

export function importFixtureOperations(
  scene: string,
  result: (data: Record<string, unknown>) => object,
) {
  return {
    ChooseInventoryImport: async () => ({
      ...result({ import_items: importFixture(scene) }),
      filename: 'planilha-modelo.xlsx',
    }),
    ReviewInventoryImport: async (
      _p: string,
      _a: string,
      rows: { line: number; ignored?: boolean }[],
    ) =>
      result({
        import_items: importFixture(scene).map((item, i) => ({
          ...item,
          row: rows[i],
          status: rows[i].ignored ? 'ignored' : item.status,
        })),
      }),
    ConfirmInventoryImport: async () =>
      result({ import_receipt: { created: 7, ignored: 5 } }),
    SaveInventoryTemplate: async () => 'OK',
  };
}
