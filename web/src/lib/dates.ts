// Date helpers. A "day" is a `YYYY-MM-DD` string in the account's time zone.
//
// Every `Date` the interface works with is a "wall date": its local fields (hours, date, …) are the
// wall-clock reading in the account's time zone, whatever zone the device is in. `wall()` turns an
// instant from the API into one and `toUtc()` turns one back, so the rest of the code can keep
// using plain local getters. When the account follows the device the two are the same thing.
//
// Arithmetic on days goes through local calendar fields, never through fixed 24-hour steps, so it
// stays correct across daylight saving transitions.

export const deviceZone = (): string => Intl.DateTimeFormat().resolvedOptions().timeZone;

let zone: string | null = null;            // null: the device's own zone, no conversion needed
let fields: Intl.DateTimeFormat | null = null;

/** Chooses the zone everything is shown in. */
export function setZone(next: string): void {
  zone = next && next !== deviceZone() ? next : null;
  fields = zone ? new Intl.DateTimeFormat('en-US', { timeZone: zone, hourCycle: 'h23', year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric' }) : null;
}

/** The wall-clock reading of an instant in the account's zone, as UTC milliseconds. */
function wallAsUtc(instant: number): number {
  const part: Record<string, number> = {};
  for (const { type, value } of fields!.formatToParts(instant)) if (type !== 'literal') part[type] = Number(value);
  return Date.UTC(part.year!, part.month! - 1, part.day!, part.hour!, part.minute!, part.second!);
}

/** An instant (a `Date` or an API timestamp) as a wall date. */
export function wall(input: string | Date): Date {
  const instant = new Date(input);
  if (!zone) return instant;
  const w = new Date(wallAsUtc(instant.getTime()));
  return new Date(w.getUTCFullYear(), w.getUTCMonth(), w.getUTCDate(), w.getUTCHours(), w.getUTCMinutes(), w.getUTCSeconds());
}

/** The current moment as a wall date. */
export const wallNow = (): Date => wall(new Date());

/** The real instant a wall date stands for. */
export function instantOf(date: Date): Date {
  if (!zone) return date;
  const target = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), date.getHours(), date.getMinutes(), date.getSeconds());
  // The zone's offset depends on the instant we are looking for; two rounds settle it, DST included.
  let guess = target;
  for (let round = 0; round < 2; round++) guess = target - (wallAsUtc(guess) - guess);
  return new Date(guess);
}

const pad = (n: number) => String(n).padStart(2, '0');

export function ymd(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export function isYmd(text: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(text)) return false;
  return ymd(parseYmd(text)) === text;
}

/** Local midnight at the start of that day. */
export function parseYmd(day: string): Date {
  const [y, m, d] = day.split('-').map(Number);
  return new Date(y!, m! - 1, d!);
}

export function addDays(day: string, n: number): string {
  const date = parseYmd(day);
  date.setDate(date.getDate() + n);
  return ymd(date);
}

export function addMonths(day: string, n: number): string {
  const date = parseYmd(day);
  const target = new Date(date.getFullYear(), date.getMonth() + n, 1);
  const last = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate();
  target.setDate(Math.min(date.getDate(), last));
  return ymd(target);
}

/** Whole days from `a` to `b`; negative when `b` is earlier. */
export function daysBetween(a: string, b: string): number {
  const [ay, am, ad] = a.split('-').map(Number);
  const [by, bm, bd] = b.split('-').map(Number);
  return Math.round((Date.UTC(by!, bm! - 1, bd!) - Date.UTC(ay!, am! - 1, ad!)) / 86_400_000);
}

/** 0 for Monday … 6 for Sunday. */
export function mondayIndex(day: string): number {
  return (parseYmd(day).getDay() + 6) % 7;
}

export function startOfWeek(day: string): string {
  return addDays(day, -mondayIndex(day));
}

export function weekDays(day: string): string[] {
  const first = startOfWeek(day);
  return Array.from({ length: 7 }, (_, k) => addDays(first, k));
}

/** ISO 8601 week number and the year that week belongs to. */
export function isoWeek(day: string): { year: number; week: number } {
  const thursday = addDays(startOfWeek(day), 3);
  const year = Number(thursday.slice(0, 4));
  return { year, week: Math.floor(daysBetween(`${year}-01-01`, thursday) / 7) + 1 };
}

export function monthStart(day: string): string {
  return `${day.slice(0, 7)}-01`;
}

export function monthEnd(day: string): string {
  const date = parseYmd(day);
  return ymd(new Date(date.getFullYear(), date.getMonth() + 1, 0));
}

export interface GridCell {
  date: string;
  inMonth: boolean;
}

/** The weeks that cover a month, Monday first, padded with days of the neighbouring months. */
export function monthGrid(day: string): GridCell[] {
  const first = monthStart(day);
  const offset = mondayIndex(first);
  const days = Number(monthEnd(day).slice(8));
  const cells = Math.ceil((offset + days) / 7) * 7;
  return Array.from({ length: cells }, (_, k) => {
    const date = addDays(first, k - offset);
    return { date, inMonth: k >= offset && k < offset + days };
  });
}

/** RFC 3339 in UTC without fractional seconds, the format the API uses. */
export function toUtc(date: Date): string {
  return instantOf(date).toISOString().replace(/\.\d{3}Z$/, 'Z');
}

/** The wall date of a wall-clock time (`HH:MM`) on a day. */
export function atLocal(day: string, time: string): Date {
  const [y, m, d] = day.split('-').map(Number);
  const [h, min] = time.split(':').map(Number);
  return new Date(y!, m! - 1, d!, h!, min!);
}

/** The wall date `minutes` of wall-clock time after midnight of a day. */
export function atMinutes(day: string, minutes: number): Date {
  const [y, m, d] = day.split('-').map(Number);
  return new Date(y!, m! - 1, d!, 0, minutes);
}

export function hhmm(date: Date): string {
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function minutesOfDay(date: Date): number {
  return date.getHours() * 60 + date.getMinutes();
}

/** UTC bounds of the days `from` (inclusive) to `to` (exclusive). */
export function utcRange(from: string, to: string): [string, string] {
  return [toUtc(parseYmd(from)), toUtc(parseYmd(to))];
}

export interface HourSpan {
  s: number;
  e: number;
}

/**
 * The part of an interval that falls on a day, as wall-clock hours from midnight (0–24).
 * `null` when the interval does not touch that day.
 */
export function spanOnDay(start: Date, end: Date, day: string): HourSpan | null {
  const dayStart = parseYmd(day);
  const dayEnd = parseYmd(addDays(day, 1));
  if (end <= dayStart || start >= dayEnd) {
    // A zero-length entry still belongs to the day it starts on.
    if (!(start.getTime() === end.getTime() && start >= dayStart && start < dayEnd)) return null;
  }
  const s = start < dayStart ? 0 : minutesOfDay(start) / 60;
  const e = end >= dayEnd ? 24 : minutesOfDay(end) / 60;
  return { s, e: Math.max(e, s) };
}

export function snap(minutes: number, step = 15): number {
  return Math.round(minutes / step) * step;
}
