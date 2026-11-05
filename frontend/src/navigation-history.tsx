import type { RouteName } from './desktop-model';

export type NavigationStep = { route: RouteName; device: string };
export function sameStep(a: NavigationStep, b: NavigationStep): boolean {
  return a.route === b.route && a.device === b.device;
}
export class NavigationHistory {
  private steps: NavigationStep[] = [];
  constructor(private readonly limit = 50) {}
  reset(step?: NavigationStep) {
    this.steps = step ? [step] : [];
  }
  record(step: NavigationStep) {
    if (this.steps.length && sameStep(this.steps[this.steps.length - 1], step))
      return;
    this.steps.push(step);
    if (this.steps.length > this.limit) this.steps.shift();
  }
  back(): NavigationStep | undefined {
    if (this.steps.length < 2) return undefined;
    this.steps.pop();
    return this.steps[this.steps.length - 1];
  }
}
