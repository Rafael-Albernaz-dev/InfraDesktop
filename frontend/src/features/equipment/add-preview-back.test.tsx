import { useCallback, useRef, useState } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AddEquipment } from './add-equipment';
import { Button } from '../../shared/ui/Button';
import { NavigationGuard } from '../../app/navigation-guard';
import type { NavigationAttempt } from '../../app/navigation-guard';
import { useNavigationHistory } from '../../app/use-navigation-history';
import { inventoryImportAPI } from '../../platform/wails/presentation';
import type { useEquipmentState } from './equipment-state';

function Harness() {
  const [root, setRoot] = useState<HTMLElement | null>(null);
  const [route, setRoute] = useState<'equipment' | 'adicionar'>('equipment');
  const guardRef = useRef<NavigationAttempt | undefined>(undefined);
  const register = useCallback((attempt?: NavigationAttempt) => {
    guardRef.current = attempt;
  }, []);
  const attempt = useCallback((action: () => void) => {
    if (guardRef.current) guardRef.current(action);
    else action();
  }, []);
  useNavigationHistory({
    root,
    scope: 'company|activation',
    authenticated: true,
    step: { route, device: '' },
    blocked: false,
    restore: (step) =>
      setRoute(step.route === 'adicionar' ? 'adicionar' : 'equipment'),
    attempt,
  });
  const s = {
    busy: false,
    credentials: [],
    credentialDefaults: {},
    setCredentialForm: vi.fn(),
    perform: vi.fn(),
    showSavedDevice: vi.fn(),
    reloadCredentials: vi.fn(),
  } as unknown as ReturnType<typeof useEquipmentState>;
  return (
    <NavigationGuard.Provider value={register}>
      <main ref={setRoot}>
        {route === 'equipment' ? (
          <Button onClick={() => setRoute('adicionar')}>Abrir</Button>
        ) : (
          <AddEquipment
            s={s}
            profile="company"
            activation="activation"
            locked={false}
            back={() => setRoute('equipment')}
          />
        )}
      </main>
    </NavigationGuard.Provider>
  );
}

it('asks before leaving a preview with all three back triggers', async () => {
  vi.spyOn(inventoryImportAPI, 'choose').mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    filename: 'fixture.csv',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      import_items: [
        {
          row: {
            line: 2,
            name: 'A',
            address: '192.0.2.1',
            port: '22',
            role: 'AP',
            block: '',
            credential: '',
          },
          status: 'ready',
        },
      ],
    },
  });
  render(<Harness />);
  fireEvent.click(screen.getByRole('button', { name: 'Abrir' }));
  fireEvent.click(screen.getByRole('tab', { name: 'Importar arquivo' }));
  fireEvent.click(screen.getByRole('button', { name: 'Escolher arquivo' }));
  await screen.findByRole('table', { name: 'Prévia da importação' });
  const doc = screen.getByRole('main').ownerDocument;
  for (const trigger of [
    () => fireEvent.keyDown(doc, { key: 'ArrowLeft', altKey: true }),
    () => fireEvent.keyDown(doc, { key: 'Backspace' }),
    () => fireEvent.mouseDown(doc, { button: 3 }),
  ]) {
    trigger();
    const dialog = screen.getByRole('dialog', {
      name: 'Descartar alterações?',
    });
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Continuar editando' }),
    );
    expect(
      screen.getByRole('table', { name: 'Prévia da importação' }),
    ).toBeVisible();
  }
  fireEvent.mouseDown(doc, { button: 3 });
  fireEvent.click(screen.getByRole('button', { name: 'Descartar alterações' }));
  expect(screen.getByRole('button', { name: 'Abrir' })).toBeVisible();
});
