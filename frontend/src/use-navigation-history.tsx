import { useEffect, useLayoutEffect, useRef } from 'react';
import { createNavigationHistory, sameStep } from './navigation-history';
import type { NavigationStep } from './navigation-history';

type NavigationState = {
  root: HTMLElement | null;
  scope: string;
  authenticated: boolean;
  step: NavigationStep;
  blocked: boolean;
  restore: (step: NavigationStep) => void;
};
function editable(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    !!target.closest(
      'input, textarea, select, [contenteditable]:not([contenteditable="false"])',
    )
  );
}
export function useNavigationHistory(state: NavigationState) {
  const history = useRef(createNavigationHistory());
  const scope = useRef('');
  const pending = useRef<
    { target: NavigationStep; origin: NavigationStep } | undefined
  >(undefined);
  const current = useRef(state);
  const {
    authenticated,
    scope: nextScope,
    step: { route, device },
  } = state;
  useLayoutEffect(() => {
    current.current = state;
  });
  useLayoutEffect(() => {
    const step = { route, device };
    if (!authenticated || scope.current !== nextScope) {
      scope.current = authenticated ? nextScope : '';
      pending.current = undefined;
      history.current.reset(
        authenticated && route !== 'companies' ? step : undefined,
      );
      return;
    }
    if (pending.current) {
      const { target, origin } = pending.current;
      if (sameStep(target, step)) {
        pending.current = undefined;
        return;
      }
      if (target.device && target.route === step.route && !step.device) return;
      pending.current = undefined;
      history.current.record(origin);
    }
    history.current.record(step);
  }, [nextScope, authenticated, route, device]);
  useEffect(() => {
    const target = state.root?.ownerDocument;
    if (!target) return;
    const back = () => {
      const s = current.current;
      if (
        !s.authenticated ||
        s.blocked ||
        s.root?.querySelector('[role="dialog"]')
      )
        return;
      const previous = history.current.back();
      if (!previous) return;
      pending.current = { target: previous, origin: s.step };
      s.restore(previous);
    };
    const key = (event: KeyboardEvent) => {
      const isBack = event.key === 'Backspace' && !editable(event.target);
      const isAltLeft =
        event.key === 'ArrowLeft' &&
        event.altKey &&
        !event.ctrlKey &&
        !event.metaKey;
      if (!isBack && !isAltLeft) return;
      event.preventDefault();
      if (event.repeat) return;
      back();
    };
    const mouse = (event: MouseEvent) => {
      if (event.button !== 3) return;
      event.preventDefault();
      back();
    };
    target.addEventListener('keydown', key);
    target.addEventListener('mousedown', mouse);
    return () => {
      target.removeEventListener('keydown', key);
      target.removeEventListener('mousedown', mouse);
    };
  }, [state.root]);
}
