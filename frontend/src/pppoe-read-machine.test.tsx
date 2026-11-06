import { expect, it } from 'vitest';
import { initialReadState as idle, readReducer } from './pppoe-read-machine';
import type { PPReadResult } from './presentation';
const result = {
  scope: 'traffic',
  rows: [],
  started_at: 'synthetic',
  ended_at: 'synthetic',
  duration_ns: 0,
} as PPReadResult;
it('requests a new window, accepts its job and sample, then completes', () => {
  const requested = readReducer(idle, {
    type: 'solicitado',
    scope: 'traffic',
    preserve: false,
  });
  expect(requested).toMatchObject({
    phase: 'solicitado',
    pending: true,
    starting: true,
    scope: 'traffic',
    result: undefined,
  });
  const running = readReducer(requested, { type: 'iniciado', job: 'job' });
  expect(running).toMatchObject({
    phase: 'iniciado',
    job: 'job',
    resultJob: 'job',
    pending: false,
    starting: false,
  });
  const sampled = readReducer(running, {
    type: 'amostra',
    result,
    next: 'next',
  });
  expect(sampled).toMatchObject({ phase: 'amostra', result, next: 'next' });
  expect(readReducer(sampled, { type: 'concluído' })).toMatchObject({
    phase: 'concluído',
    job: '',
    result,
    pending: false,
  });
  expect(idle.result).toBeUndefined();
});
it('renews and pauses without erasing points; a manual new request clears them', () => {
  const sampled = { ...idle, result, job: 'old' };
  expect(readReducer(sampled, { type: 'renovando' })).toMatchObject({
    phase: 'renovando',
    result,
    renewing: true,
    job: '',
  });
  expect(readReducer(sampled, { type: 'pausado' })).toMatchObject({
    phase: 'pausado',
    result,
    renewing: false,
    job: '',
  });
  expect(
    readReducer(sampled, {
      type: 'solicitado',
      scope: 'traffic',
      preserve: true,
    }).result,
  ).toBe(result);
  expect(
    readReducer(sampled, {
      type: 'solicitado',
      scope: 'traffic',
      preserve: false,
    }).result,
  ).toBeUndefined();
});
it('keeps failure evidence and reconciles pending operations explicitly', () => {
  const failure = readReducer(
    { ...idle, pending: true, result },
    { type: 'falhou', error: 'Falhou', reason: 'synthetic' },
  );
  expect(failure).toMatchObject({
    phase: 'falhou',
    error: 'Falhou',
    failureReason: 'synthetic',
    pending: true,
    result,
  });
  expect(
    readReducer(failure, { type: 'falhou', error: 'Recusado', pending: false })
      .pending,
  ).toBe(false);
});
it('operator stop releases the UI and retains the last result', () => {
  expect(
    readReducer(
      { ...idle, job: 'job', result, renewing: true, cancelling: true },
      { type: 'parado' },
    ),
  ).toMatchObject({
    phase: 'parado',
    job: '',
    pending: false,
    renewing: false,
    cancelling: false,
    stoppedTraffic: true,
    result,
  });
});
