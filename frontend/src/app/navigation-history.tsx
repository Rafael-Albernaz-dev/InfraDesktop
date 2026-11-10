import type { RouteName } from '../features/access/desktop-model';
import type { SessionRouteTarget } from '../features/equipment/session-navigation';

export type NavigationStep = {
  route: RouteName;
  device: string;
  session?: SessionRouteTarget;
};
export function sameStep(a: NavigationStep, b: NavigationStep): boolean {
  return (
    a.route === b.route &&
    a.device === b.device &&
    a.session?.row === b.session?.row &&
    a.session?.observation === b.session?.observation
  );
}
export function createNavigationHistory(limit = 50) {
  let steps: NavigationStep[] = [];
  return {
    reset(step?: NavigationStep) {
      steps = step ? [step] : [];
    },
    record(step: NavigationStep) {
      if (steps.length && sameStep(steps[steps.length - 1], step)) return;
      const origin = step.session?.originDevice;
      const previous = steps[steps.length - 1];
      const before = steps[steps.length - 2];
      if (
        origin &&
        previous?.route === 'equipment' &&
        previous.device === step.session?.device &&
        before?.route === 'equipment' &&
        before.device === origin
      )
        steps.pop();
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
