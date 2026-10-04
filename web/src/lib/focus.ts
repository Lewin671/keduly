// The arithmetic of the focus timer: countdowns, tomatoes against estimates, and the statistics chart.
import type { FocusDay, FocusSession } from '../api/types';
import { wall, ymd } from './dates';

/** A countdown as `MM:SS`. A second that has begun still shows. */
export function mmss(seconds: number): string {
  const s = Math.max(0, Math.ceil(seconds));
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
}

/** Seconds until the session ends, by the server's clock (`nowMs`). Never negative. */
export function secondsLeft(session: Pick<FocusSession, 'end'>, nowMs: number): number {
  return Math.max(0, (Date.parse(session.end) - nowMs) / 1000);
}

/** The share of the session still to run, 1 at the start and 0 at the end. */
export function fractionLeft(session: Pick<FocusSession, 'end' | 'planned_minutes'>, nowMs: number): number {
  return Math.min(1, secondsLeft(session, nowMs) / (session.planned_minutes * 60));
}

/** Minutes the session has lasted so far: up to now while it runs, its whole length afterwards. */
export function minutesSpent(session: Pick<FocusSession, 'start' | 'end'>, nowMs: number): number {
  return Math.max(0, (Math.min(Date.parse(session.end), nowMs) - Date.parse(session.start)) / 60_000);
}

/** An estimate expressed in tomatoes; 0 without an estimate. */
export function tomatoesNeeded(estimateMinutes: number | null, tomatoMinutes: number): number {
  return estimateMinutes ? Math.ceil(estimateMinutes / tomatoMinutes) : 0;
}

/** The top of the chart's scale: an even number, at least 4, that holds the busiest day. */
export function chartScale(days: readonly Pick<FocusDay, 'tomatoes'>[]): number {
  return Math.max(4, Math.ceil(Math.max(0, ...days.map(d => d.tomatoes)) / 2) * 2);
}

export interface SessionDay {
  date: string;
  sessions: FocusSession[];
}

/** Sessions that have ended, grouped by the local day they started on: newest day and newest session first. */
export function sessionsByDay(sessions: readonly FocusSession[], nowMs: number): SessionDay[] {
  const days = new Map<string, FocusSession[]>();
  for (const s of sessions) {
    if (Date.parse(s.end) > nowMs) continue;
    const date = ymd(wall(s.start));
    days.set(date, [...(days.get(date) ?? []), s]);
  }
  return [...days.entries()]
    .sort((a, b) => (a[0] < b[0] ? 1 : -1))
    .map(([date, list]) => ({ date, sessions: list.sort((a, b) => Date.parse(b.start) - Date.parse(a.start)) }));
}
