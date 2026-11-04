export type RefreshAttempt = { state: string; code: string; at: string };
export function latestRefreshAttempt(
  ...attempts: (RefreshAttempt | undefined)[]
): RefreshAttempt | undefined {
  let latest: RefreshAttempt | undefined;
  for (const attempt of attempts) {
    if (!attempt) continue;
    if (!latest || Date.parse(attempt.at) >= Date.parse(latest.at))
      latest = attempt;
  }
  return latest;
}

export function refreshAttemptInProgress(attempt?: { state: string }): boolean {
  return attempt?.state === 'RUNNING' || attempt?.state === 'STARTED';
}
