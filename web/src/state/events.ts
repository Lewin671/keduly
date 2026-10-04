// Calendar events are fetched for the visible range; the ranges next to it are fetched ahead
// so that paging feels instant.
import { signal } from '@preact/signals';
import { useEffect } from 'preact/hooks';
import * as api from '../api/client';
import type { CalEvent } from '../api/types';
import { utcRange } from '../lib/dates';
import { track } from './resource';
import { onSessionEnd, reportError, version } from './store';

/** Local days: `from` inclusive, `to` exclusive. */
export type DayRange = readonly [from: string, to: string];

interface Entry {
  events: CalEvent[];
  v: number;
}

const cache = signal<Record<string, Entry>>({});
const pending = new Map<string, Promise<void>>();
const keyOf = (range: DayRange) => `${range[0]}/${range[1]}`;

onSessionEnd(() => { cache.value = {}; });

function ensure(range: DayRange, v: number, visible: boolean): Promise<void> {
  const key = keyOf(range);
  if (cache.value[key]?.v === v) return Promise.resolve();
  const id = `${key}@${v}`;
  let work = pending.get(id);
  if (!work) {
    const load = api.listEvents(...utcRange(range[0], range[1])).then(
      events => { if (v === version.value) cache.value = { ...cache.value, [key]: { events, v } }; },
      err => { if (visible) reportError(err); },
    );
    work = (visible ? track(load) : load).finally(() => { pending.delete(id); });
    pending.set(id, work);
  }
  return work;
}

/** Events of the visible range; stale ones are shown while fresher ones load. */
export function useEvents(range: DayRange, neighbours: readonly DayRange[]): CalEvent[] | undefined {
  const key = keyOf(range);
  const near = neighbours.map(keyOf).join(',');
  const v = version.value;
  useEffect(() => {
    let live = true;
    void ensure(range, v, true).then(() => {
      if (!live) return;
      const wanted = new Set([key, ...neighbours.map(keyOf)]);
      cache.value = Object.fromEntries(Object.entries(cache.value).filter(([k]) => wanted.has(k)));
      for (const n of neighbours) void ensure(n, v, false);
    });
    return () => { live = false; };
  }, [key, near, v]);
  return cache.value[key]?.events;
}

export const eventKey = (ev: CalEvent) => `${ev.id}|${ev.instance ?? ''}|${ev.status}|${ev.suggestion_id ?? ''}`;

/** Changes cached events at once, ahead of the server. Returns a function that puts them back. */
export function patchEvents(change: (ev: CalEvent) => CalEvent | null): () => void {
  const before = cache.value;
  cache.value = Object.fromEntries(
    Object.entries(before).map(([key, entry]) => [
      key,
      { ...entry, events: entry.events.map(change).filter((ev): ev is CalEvent => ev !== null) },
    ]),
  );
  return () => { cache.value = before; };
}
