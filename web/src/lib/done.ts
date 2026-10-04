// The log of finished items is shown newest first, one section per month of completion.
import { ymd, wall } from './dates';

export interface MonthGroup<T> {
  /** `YYYY-MM` in local time. */
  month: string;
  items: T[];
}

export function groupByMonth<T extends { completed_at: string | null }>(items: readonly T[]): MonthGroup<T>[] {
  const groups: MonthGroup<T>[] = [];
  for (const item of items) {
    // Completed items always carry a timestamp; one without it is kept with its neighbours.
    const month = item.completed_at ? ymd(wall(item.completed_at)).slice(0, 7) : (groups.at(-1)?.month ?? '');
    const last = groups.at(-1);
    if (last && last.month === month) last.items.push(item);
    else groups.push({ month, items: [item] });
  }
  return groups;
}
