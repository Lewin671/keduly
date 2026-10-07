// How events map onto days, and how the calendar pages through time.
import type { CalEvent, FocusSession } from '../api/types';
import { addDays, addMonths, isoWeek, monthStart, spanOnDay, startOfWeek, type HourSpan, wall } from './dates';

export type CalView = 'day' | 'week' | 'month' | 'year';

/** The part of a timed event on a day, in hours from midnight; `null` for all-day events and other days. */
export function eventSpan(event: CalEvent, day: string): HourSpan | null {
  if (event.all_day || !event.start || !event.end) return null;
  return spanOnDay(wall(event.start), wall(event.end), day);
}

/** A session shorter than this stays a line: a block is always drawn taller than that. */
const BLOCK_MS = 10 * 60_000;

/**
 * Whether a session is drawn as a block of its own. Beside its item's time block a line is
 * enough; where nothing was planned, the block is all that says what the time went to.
 */
export function standsAlone(s: Pick<FocusSession, 'item_id' | 'start'>, endMs: number, events: readonly CalEvent[]): boolean {
  const startMs = Date.parse(s.start);
  if (endMs - startMs < BLOCK_MS) return false;
  return !events.some(e => e.item_id !== null && e.item_id === s.item_id && e.status === 'confirmed' && !!e.start && !!e.end
    && Date.parse(e.start) < endMs && Date.parse(e.end) > startMs);
}

/** Whether an all-day event covers a day (its end date is inclusive). */
export function allDayOn(event: CalEvent, day: string): boolean {
  if (!event.all_day || !event.start_date) return false;
  return event.start_date <= day && (event.end_date ?? event.start_date) >= day;
}

/** Moves the current date one page in a view. */
export function stepDate(view: CalView, day: string, direction: 1 | -1): string {
  switch (view) {
    case 'day': return addDays(day, direction);
    case 'week': return addDays(day, 7 * direction);
    case 'month': return addMonths(monthStart(day), direction);
    case 'year': return addMonths(monthStart(day), 12 * direction);
  }
}

/** The local days a view needs events for (`to` exclusive), and the same for the pages next to it. */
export function viewRange(view: 'day' | 'week' | 'month', day: string): { range: [string, string]; neighbours: [string, string][] } {
  if (view === 'month') {
    const first = monthStart(day);
    const of = (n: number): [string, string] => [addMonths(first, n), addMonths(first, n + 1)];
    return { range: of(0), neighbours: [of(1), of(-1)] };
  }
  // The day view loads its whole week, so stepping through days needs no request.
  const monday = startOfWeek(day);
  const of = (n: number): [string, string] => [addDays(monday, 7 * n), addDays(monday, 7 * (n + 1))];
  return { range: of(0), neighbours: [of(1), of(-1)] };
}

/** Year, month and ISO week shown in the week view's title: those of the week's Thursday. */
export function weekTitle(day: string): { year: number; month: number; week: number } {
  const thursday = addDays(startOfWeek(day), 3);
  return { year: Number(thursday.slice(0, 4)), month: Number(thursday.slice(5, 7)), week: isoWeek(day).week };
}
