import { RadioRoles } from './radio-contract';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import { inventoryImportAPI } from './presentation';
import { NavigationGuard } from './navigation-guard';
import type { useEquipmentState } from './equipment-state';
import type { ImportRow } from './presentation';

const scope = { profile_id: 'company', activation_id: 'activation' };
const row: ImportRow = {
  line: 2,
  name: 'AP',
  address: '192.0.2.31',
  port: '22',
  role: RadioRoles.ap,
  block: '',
  credential: '',
};
function setup() {
  const back = vi.fn();
  const state = {
    busy: false,
    surfaceRef: { current: 0 },
    credentials: [
      { id: 'cred', alias: 'leitura', user: 'fixture', revision: 1, links: 0 },
    ],
    credentialDefaults: { [RadioRoles.ap]: 'cred' },
    setCredentialForm: vi.fn(),
    perform: vi.fn(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  let guard: ((action: () => void) => void) | undefined;
  render(
    <NavigationGuard.Provider
      value={(value) => {
        guard = value;
      }}
    >
      <AddEquipment
        s={state}
        profile="company"
        activation="activation"
        locked={false}
        back={back}
      />
    </NavigationGuard.Provider>,
  );
  return { back, state, navigate: (action: () => void) => guard?.(action) };
}

describe('Adicionar equipamentos', () => {
  it('uses the role default and guards unsaved navigation', () => {
    const s = setup();
    expect(screen.getByLabelText('Credencial de consulta')).toHaveValue('cred');
    fireEvent.change(screen.getByLabelText('Nome'), {
      target: { value: 'Novo AP' },
    });
    const navigate = vi.fn();
    act(() => s.navigate(navigate));
    expect(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Continuar editando' }));
    expect(navigate).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
    fireEvent.click(
      screen.getByRole('button', { name: 'Descartar alterações' }),
    );
    expect(s.back).toHaveBeenCalledOnce();
  });

  it('blocks registration while an open row remains and can ignore or restore it', async () => {
    vi.spyOn(inventoryImportAPI, 'choose').mockResolvedValue({
      ...scope,
      code: 'OK',
      filename: 'fixture.csv',
      data: {
        devices: [],
        credentials: [],
        next_cursor: 0,
        vault_available: true,
        import_items: [{ row, status: 'pending', reason: 'Corrija o campo' }],
      },
    });
    vi.spyOn(inventoryImportAPI, 'review').mockImplementation(
      async (_p, _a, rows) => ({
        ...scope,
        code: 'OK',
        data: {
          devices: [],
          credentials: [],
          next_cursor: 0,
          vault_available: true,
          import_items: rows.map((item) => ({
            row: item,
            status: item.ignored ? 'ignored' : 'pending',
            reason: item.ignored ? '' : 'Corrija o campo',
          })),
        },
      }),
    );
    setup();
    fireEvent.click(screen.getByRole('tab', { name: 'Importar arquivo' }));
    fireEvent.click(screen.getByRole('button', { name: 'Escolher arquivo' }));
    const table = await screen.findByRole('table', {
      name: 'Prévia da importação',
    });
    expect(
      screen.getByRole('button', { name: /Cadastrar 0 equipamentos/ }),
    ).toBeDisabled();
    fireEvent.click(within(table).getByRole('button', { name: 'Ignorar' }));
    await waitFor(() =>
      expect(
        within(table).getByRole('button', { name: 'Restaurar' }),
      ).toBeInTheDocument(),
    );
    fireEvent.click(within(table).getByRole('button', { name: 'Restaurar' }));
    await waitFor(() =>
      expect(within(table).getByText('Pendente')).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
    expect(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).toBeInTheDocument();
  });
});

it('sends corrected and selected rows back for service review', async () => {
  const second = { ...row, line: 3, name: 'AP B', address: '192.0.2.32' };
  const result = (rows: ImportRow[]) => ({
    ...scope,
    code: 'OK',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      import_items: rows.map((item) => ({
        row: item,
        status:
          item.address === 'invalid'
            ? ('pending' as const)
            : ('ready' as const),
        reason: item.address === 'invalid' ? 'IP inválido' : '',
      })),
    },
  });
  vi.spyOn(inventoryImportAPI, 'choose').mockResolvedValue({
    ...result([{ ...row, address: 'invalid' }, second]),
    filename: 'fixture.csv',
  });
  const review = vi
    .spyOn(inventoryImportAPI, 'review')
    .mockImplementation(async (_p, _a, rows) => result(rows));
  setup();
  fireEvent.click(screen.getByRole('tab', { name: 'Importar arquivo' }));
  fireEvent.click(screen.getByRole('button', { name: 'Escolher arquivo' }));
  const table = await screen.findByRole('table', {
    name: 'Prévia da importação',
  });
  fireEvent.click(
    within(table).getAllByRole('button', { name: 'Corrigir' })[0],
  );
  const dialog = screen.getByRole('dialog', { name: 'Corrigir linha 2' });
  expect(within(dialog).getByText('IP inválido')).toBeInTheDocument();
  fireEvent.change(within(dialog).getByLabelText('IP de gerenciamento'), {
    target: { value: '192.0.2.31' },
  });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Salvar linha' }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Cadastrar 2 equipamentos' }),
    ).toBeEnabled(),
  );
  fireEvent.click(within(table).getByLabelText('Selecionar linha 2'));
  fireEvent.click(screen.getByRole('button', { name: 'Definir bloco' }));
  const bulk = screen.getByRole('dialog', { name: 'Definir bloco' });
  fireEvent.change(within(bulk).getByLabelText('Bloco'), {
    target: { value: 'Torre fixture' },
  });
  fireEvent.click(within(bulk).getByRole('button', { name: 'Aplicar' }));
  await waitFor(() =>
    expect(review).toHaveBeenLastCalledWith(
      'company',
      'activation',
      expect.arrayContaining([
        expect.objectContaining({ line: 2, block: 'Torre fixture' }),
      ]),
    ),
  );
});
