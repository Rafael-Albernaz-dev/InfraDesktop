// Deterministic replies for the same approved, synthetic snapshot used by GetRadio.
export function radioFixtureOperations(
  result: (data: object) => unknown,
  reading: (id: string) => unknown,
) {
  const jobs = new Map<string, string>();
  return {
    StartRadio: async (...args: [string, string, { device_id: string }]) => {
      const input = args[2];
      const id = `fixture-radio-${input.device_id}`;
      jobs.set(id, input.device_id);
      return result({ job: { id, state: 'RUNNING', code: 'OK' } });
    },
    RadioStatus: async (...args: [string, string, string]) =>
      result({
        job: { id: args[2], state: 'SUCCEEDED', code: 'OK' },
        reading: reading(jobs.get(args[2]) || ''),
      }),
    CancelRadioOperation: async () => result({}),
  };
}
