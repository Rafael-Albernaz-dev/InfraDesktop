import type { SetStateAction } from 'react';
import type { ReadContext } from './pppoe-read-lifecycle';

export function retryReadStatus(c: ReadContext, value: SetStateAction<number>) {
  const l = c.life.current;
  if (l.disposed || c.state.current.cancelling || !l.activeJob) return;
  // An explicit status check grants a fresh read of this job, never a new collection.
  l.jobGeneration = ++l.generation;
  c.send({
    type: 'metadata',
    values: {
      pollRetry:
        typeof value === 'function' ? value(c.state.current.pollRetry) : value,
    },
  });
}
