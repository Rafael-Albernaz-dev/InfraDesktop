import { presentationMock } from './presentation-mock';
export function radioAPIMock(
  overrides: { radioAPI?: Record<string, unknown> } = {},
) {
  return {
    radioAPI: presentationMock({ radioAPI: overrides.radioAPI ?? {} }).radioAPI,
  };
}
