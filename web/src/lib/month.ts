// What a month cell shows when the day has more entries than rows.

export interface MonthEntry {
  allDay: boolean;
  /** A tentative or leaving entry of a pending suggestion. */
  pending: boolean;
  /** Start as hours from midnight; ignored for all-day entries. */
  s: number;
}

export const MONTH_ROWS = 4;

/**
 * At most `max` rows. A crowded day gives its last row to "N more" and keeps what matters most:
 * all-day events, then pending suggestions, then the rest by time. The kept entries are shown
 * with all-day ones first, then by start time.
 */
export function pickMonthEntries<T extends MonthEntry>(list: readonly T[], max = MONTH_ROWS): { visible: T[]; more: number } {
  const rank = (x: T) => (x.allDay ? 0 : x.pending ? 1 : 2);
  const start = (x: T) => (x.allDay ? 0 : x.s);
  const keep = list.length > max ? max - 1 : max;
  const visible = list
    .map((entry, index) => ({ entry, index }))
    .sort((a, b) => rank(a.entry) - rank(b.entry) || start(a.entry) - start(b.entry) || a.index - b.index)
    .slice(0, keep)
    .sort((a, b) => Number(b.entry.allDay) - Number(a.entry.allDay) || start(a.entry) - start(b.entry) || a.index - b.index)
    .map(x => x.entry);
  return { visible, more: list.length - visible.length };
}

/** Busyness level of a day in the year view: 0 (nothing) to 3. */
export function heatLevel(count: number): 0 | 1 | 2 | 3 {
  return count >= 5 ? 3 : count >= 3 ? 2 : count >= 1 ? 1 : 0;
}
