import type { RouteName } from './desktop-model';

export type NavigationStep = { route: RouteName; device: string };
export function sameStep(a: NavigationStep, b: NavigationStep): boolean {
  return a.route === b.route && a.device === b.device;
}
export function createNavigationHistory(limit = 50) {
  let steps: NavigationStep[] = [];
  return {
    reset(step?: NavigationStep) {
      steps = step ? [step] : [];
    },
    record(step: NavigationStep) {
      if (steps.length && sameStep(steps[steps.length - 1], step)) return;
      steps.push(step);
      if (steps.length > limit) steps.shift();
    },
    back(): NavigationStep | undefined {
      if (steps.length < 2) return undefined;
      steps.pop();
      return steps[steps.length - 1];
    },
  };
}
