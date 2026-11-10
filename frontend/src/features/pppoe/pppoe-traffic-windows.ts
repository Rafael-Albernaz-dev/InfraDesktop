import type { PPReadResult } from '../../platform/wails/presentation';

// Preserve only the last five minutes of plotted points; each new window has its own clock origin.
export function mergeTrafficWindows(
  previous: PPReadResult | undefined,
  next: PPReadResult,
): PPReadResult {
  if (!previous || previous.scope !== 'traffic' || next.scope !== 'traffic')
    return next;
  const previousStart = Date.parse(previous.started_at);
  const nextStart = Date.parse(next.started_at);
  const rows = next.rows.map((row) => {
    const old = previous.rows.find((item) => item.row_id === row.row_id);
    if (!old?.traffic || !row.traffic) return row;
    const prior = old.traffic.points;
    const last = prior.at(-1)?.t ?? 0;
    const first = row.traffic.points[0]?.t ?? 0;
    const elapsed =
      Number.isFinite(previousStart) && Number.isFinite(nextStart)
        ? nextStart - previousStart
        : last + 2;
    const offset = Math.max(elapsed, last + 2 - first);
    const following = row.traffic.points.map((point) => ({
      ...point,
      t: point.t + offset,
    }));
    const gap =
      prior.length && following.length
        ? [{ t: (last + following[0].t) / 2, gap: true }]
        : [];
    const points = [...prior, ...gap, ...following];
    const cutoff = (points.at(-1)?.t ?? 0) - 300000;
    return {
      ...row,
      traffic: {
        ...row.traffic,
        points: points.filter((point) => point.t >= cutoff),
        gaps: row.traffic.gaps + gap.length,
      },
    };
  });
  return { ...next, started_at: previous.started_at, rows };
}
