import { presentationMock } from './presentation-mock';
export function equipmentAPIMock(
  overrides: {
    equipmentAPI?: Record<string, unknown>;
    equipmentMessage?: (code: string) => string;
  } = {},
) {
  return {
    equipmentAPI: presentationMock({
      equipmentAPI: overrides.equipmentAPI ?? {},
    }).equipmentAPI,
    equipmentMessage: overrides.equipmentMessage ?? ((code: string) => code),
  };
}
