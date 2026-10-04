import { describe, expect, it } from 'vitest';
import { heatLevel, pickMonthEntries, type MonthEntry } from './month';

interface Named extends MonthEntry { name: string }
const timed = (name: string, s: number, pending = false): Named => ({ name, s, pending, allDay: false });
const allDay = (name: string): Named => ({ name, s: 0, pending: false, allDay: true });
const names = (list: Named[]) => list.map(x => x.name);

describe('month cell', () => {
  it('shows up to four entries in full', () => {
    const list = [timed('c', 15), timed('a', 9), allDay('holiday'), timed('b', 10)];
    const { visible, more } = pickMonthEntries(list);
    expect(names(visible)).toEqual(['holiday', 'a', 'b', 'c']);
    expect(more).toBe(0);
  });

  it('gives the fourth row to "N more" when there are five or more', () => {
    const list = [timed('a', 9), timed('b', 10), timed('c', 11), timed('d', 12), timed('e', 13)];
    const { visible, more } = pickMonthEntries(list);
    expect(names(visible)).toEqual(['a', 'b', 'c']);
    expect(more).toBe(2);
  });

  it('keeps all-day events and pending suggestions on a crowded day', () => {
    const list = [timed('a', 9), timed('b', 10), timed('c', 11), timed('late pending', 16, true), allDay('conference'), timed('d', 12)];
    const { visible, more } = pickMonthEntries(list);
    // Kept by rank, then shown with the all-day entry first and the rest by time.
    expect(names(visible)).toEqual(['conference', 'a', 'late pending']);
    expect(more).toBe(3);
  });

  it('prefers pending entries over earlier confirmed ones', () => {
    const list = [timed('a', 8), timed('b', 9), timed('c', 10), timed('p1', 14, true), timed('p2', 15, true), timed('p3', 16, true)];
    expect(names(pickMonthEntries(list).visible)).toEqual(['p1', 'p2', 'p3']);
  });

  it('keeps the given order for entries that tie', () => {
    const list = [allDay('first'), allDay('second'), timed('x', 9), timed('y', 9)];
    expect(names(pickMonthEntries(list).visible)).toEqual(['first', 'second', 'x', 'y']);
  });

  it('copes with an empty day', () => {
    expect(pickMonthEntries([])).toEqual({ visible: [], more: 0 });
  });
});

describe('year heat', () => {
  it('maps a day count to a level', () => {
    expect([0, 1, 2, 3, 4, 5, 40].map(heatLevel)).toEqual([0, 1, 1, 2, 2, 3, 3]);
  });
});
