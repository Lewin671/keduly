// Blocks that overlap share a column side by side: each cluster of overlapping blocks gets as
// many lanes as it needs, and every block of the cluster is as wide as one lane.
import type { HourSpan } from './dates';

export interface Lane {
  lane: number;
  of: number;
}

/** Very short blocks are drawn taller than they last, so they also occupy that much time. */
const MIN_HOURS = 0.4;

export function layoutLanes<T extends HourSpan>(list: readonly T[]): Map<T, Lane> {
  const out = new Map<T, Lane>();
  const end = (x: T) => Math.max(x.e, x.s + MIN_HOURS);
  let cluster: T[] = [];
  let until = -1;
  const close = () => {
    const of = Math.max(...cluster.map(x => out.get(x)!.lane)) + 1;
    for (const x of cluster) out.get(x)!.of = of;
    cluster = [];
  };
  for (const x of [...list].sort((a, b) => a.s - b.s || b.e - a.e)) {
    if (cluster.length && x.s >= until) close();
    const taken = new Set(cluster.filter(other => end(other) > x.s).map(other => out.get(other)!.lane));
    let lane = 0;
    while (taken.has(lane)) lane++;
    out.set(x, { lane, of: 1 });
    until = cluster.length ? Math.max(until, end(x)) : end(x);
    cluster.push(x);
  }
  if (cluster.length) close();
  return out;
}
