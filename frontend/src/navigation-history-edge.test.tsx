import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { useState } from 'react';
import { Button } from './ui/Button';
import { useNavigationHistory } from './use-navigation-history';
import type { NavigationStep } from './navigation-history';

const overview: NavigationStep = { route: 'overview', device: '' };
const missing: NavigationStep = { route: 'equipment', device: 'missing' };
const settings: NavigationStep = { route: 'settings', device: '' };
const credentials: NavigationStep = { route: 'credentials', device: '' };
function Harness({
  failMissing = false,
  deferMissing = false,
  failedAfterList = false,
  restored,
}: {
  failMissing?: boolean;
  deferMissing?: boolean;
  failedAfterList?: boolean;
  restored: (step: NavigationStep) => void;
}) {
  const [root, setRoot] = useState<HTMLElement | null>(null);
  const [step, setStep] = useState(overview);
  useNavigationHistory({
    root,
    scope: 'company|activation',
    authenticated: true,
    blocked: false,
    step,
    restore: (next) => {
      restored(next);
      if (failMissing && next.device === 'missing') return;
      if ((deferMissing || failedAfterList) && next.device === 'missing') {
        setStep({ route: 'equipment', device: '' });
        return;
      }
      setStep(next);
    },
  });
  return (
    <main ref={setRoot}>
      <p role="status">
        {step.route}:{step.device}
      </p>
      <Button onClick={() => setStep(overview)}>Overview</Button>
      <Button onClick={() => setStep(missing)}>Sheet</Button>
      <Button onClick={() => setStep({ route: 'equipment', device: 'other' })}>
        Other sheet
      </Button>
      <Button onClick={() => setStep(settings)}>Settings</Button>
      <Button onClick={() => setStep(credentials)}>Credentials</Button>
    </main>
  );
}
function press(key: string, repeat = false) {
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key,
    altKey: key === 'ArrowLeft',
    repeat,
  });
}
it.each(['Backspace', 'ArrowLeft'])(
  'ignores auto-repeat for %s and moves back only once',
  (key) => {
    const restored = vi.fn();
    render(<Harness restored={restored} />);
    fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
    fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
    fireEvent.click(screen.getByRole('button', { name: 'Credentials' }));
    press(key);
    press(key, true);
    press(key, true);
    expect(screen.getByRole('status')).toHaveTextContent('settings:');
    expect(restored).toHaveBeenCalledTimes(1);
  },
);
it('records later navigation after restoring an unavailable sheet', () => {
  const restored = vi.fn();
  render(<Harness restored={restored} failMissing />);
  fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  press('Backspace');
  expect(restored).toHaveBeenLastCalledWith(missing);
  expect(screen.getByRole('status')).toHaveTextContent('settings:');
  fireEvent.click(screen.getByRole('button', { name: 'Credentials' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('credentials:');
});
it('records another sheet after an unavailable restore', () => {
  const restored = vi.fn();
  render(<Harness restored={restored} failMissing />);
  fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  press('Backspace');
  fireEvent.click(screen.getByRole('button', { name: 'Other sheet' }));
  expect(screen.getByRole('status')).toHaveTextContent('equipment:other');
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('settings:');
});
it('records new routes after an unavailable sheet leaves the list visible', () => {
  const restored = vi.fn();
  render(<Harness restored={restored} failedAfterList />);
  fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('equipment:');
  fireEvent.click(screen.getByRole('button', { name: 'Credentials' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('credentials:');
});
it('preserves the stack through an intermediate list and a successful asynchronous sheet restore', () => {
  const restored = vi.fn();
  render(<Harness restored={restored} deferMissing />);
  fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
  fireEvent.click(screen.getByRole('button', { name: 'Credentials' }));
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('settings:');
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('equipment:');
  fireEvent.click(screen.getByRole('button', { name: 'Sheet' }));
  expect(screen.getByRole('status')).toHaveTextContent('equipment:missing');
  press('Backspace');
  expect(screen.getByRole('status')).toHaveTextContent('overview:');
});
