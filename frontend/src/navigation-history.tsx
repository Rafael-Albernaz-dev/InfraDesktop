import type { RouteName } from './desktop-model';
import type { SessionRouteTarget } from './session-navigation';

export type NavigationStep = { route: RouteName; device: string; session?: SessionRouteTarget };
export function sameStep(a: NavigationStep, b: NavigationStep): boolean {
  return a.route === b.route && a.device === b.device &&
    a.session?.row === b.session?.row && a.session?.observation === b.session?.observation;
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
