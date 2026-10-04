import { describe, expect, it } from 'vitest';
import {
  addDays, addMonths, atLocal, atMinutes, daysBetween, hhmm, isoWeek, isYmd, monthEnd, monthGrid, mondayIndex, parseYmd, snap,
  spanOnDay, startOfWeek, toUtc, utcRange, weekDays, ymd,
} from './dates';

// The suite runs in America/New_York (see vite.config.ts): DST starts 2026-03-08 and ends 2026-11-01.
describe('time zone of the test run', () => {
  it('is the one the expectations below assume', () => {
    expect(Intl.DateTimeFormat().resolvedOptions().timeZone).toBe('America/New_York');
  });
});

describe('days', () => {
  it('formats and parses local days', () => {
    expect(ymd(new Date(2026, 9, 13, 23, 59))).toBe('2026-10-13');
    expect(parseYmd('2026-10-13').getHours()).toBe(0);
    expect(ymd(parseYmd('2026-01-01'))).toBe('2026-01-01');
  });

  it('takes the local day of an instant, not the UTC one', () => {
    // 03:30 UTC on the 14th is still the evening of the 13th in New York.
    expect(ymd(new Date('2026-10-14T03:30:00Z'))).toBe('2026-10-13');
    expect(hhmm(new Date('2026-10-14T03:30:00Z'))).toBe('23:30');
  });

  it('validates day strings', () => {
    expect(isYmd('2026-02-28')).toBe(true);
    expect(isYmd('2026-02-30')).toBe(false);
    expect(isYmd('2026-2-3')).toBe(false);
    expect(isYmd('today')).toBe(false);
  });

  it('adds days across month ends and DST changes', () => {
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01');
    expect(addDays('2026-03-07', 1)).toBe('2026-03-08');
    expect(addDays('2026-03-08', 1)).toBe('2026-03-09');
    expect(addDays('2026-11-01', 1)).toBe('2026-11-02');
    expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
  });

  it('adds months and clamps to the last day', () => {
    expect(addMonths('2026-01-31', 1)).toBe('2026-02-28');
    expect(addMonths('2026-12-01', 1)).toBe('2027-01-01');
    expect(addMonths('2026-03-15', -3)).toBe('2025-12-15');
  });

  it('counts whole days regardless of DST', () => {
    expect(daysBetween('2026-03-07', '2026-03-09')).toBe(2);
    expect(daysBetween('2026-10-31', '2026-11-02')).toBe(2);
    expect(daysBetween('2026-10-13', '2026-10-09')).toBe(-4);
  });

  it('finds the end of a month', () => {
    expect(monthEnd('2028-02-10')).toBe('2028-02-29');
    expect(monthEnd('2026-10-13')).toBe('2026-10-31');
  });
});

describe('weeks start on Monday', () => {
  it('indexes weekdays from Monday', () => {
    expect(mondayIndex('2026-10-12')).toBe(0);
    expect(mondayIndex('2026-10-13')).toBe(1);
    expect(mondayIndex('2026-10-18')).toBe(6);
  });

  it('gives the week of a day', () => {
    expect(startOfWeek('2026-10-18')).toBe('2026-10-12');
    expect(weekDays('2026-10-13')).toEqual(['2026-10-12', '2026-10-13', '2026-10-14', '2026-10-15', '2026-10-16', '2026-10-17', '2026-10-18']);
  });

  it('keeps seven distinct days in the weeks that contain a DST change', () => {
    expect(weekDays('2026-03-08')).toEqual(['2026-03-02', '2026-03-03', '2026-03-04', '2026-03-05', '2026-03-06', '2026-03-07', '2026-03-08']);
    expect(weekDays('2026-11-01')[6]).toBe('2026-11-01');
    expect(weekDays('2026-11-02')[0]).toBe('2026-11-02');
  });

  it('numbers weeks as ISO 8601 does', () => {
    expect(isoWeek('2026-10-13')).toEqual({ year: 2026, week: 42 });
    expect(isoWeek('2026-01-01')).toEqual({ year: 2026, week: 1 });
    expect(isoWeek('2027-01-01')).toEqual({ year: 2026, week: 53 });
    expect(isoWeek('2024-12-30')).toEqual({ year: 2025, week: 1 });
    expect(isoWeek('2021-01-03')).toEqual({ year: 2020, week: 53 });
  });
});

describe('month grid', () => {
  it('pads October 2026 to whole weeks', () => {
    const grid = monthGrid('2026-10-13');
    expect(grid).toHaveLength(35);
    expect(grid[0]).toEqual({ date: '2026-09-28', inMonth: false });
    expect(grid[3]).toEqual({ date: '2026-10-01', inMonth: true });
    expect(grid[34]).toEqual({ date: '2026-11-01', inMonth: false });
  });

  it('needs six rows when a 31-day month starts late in the week', () => {
    const grid = monthGrid('2026-08-01');
    expect(grid).toHaveLength(42);
    expect(grid[5]!.date).toBe('2026-08-01');
    expect(grid.filter(c => c.inMonth)).toHaveLength(31);
  });

  it('fits February 2027 in exactly four rows', () => {
    const grid = monthGrid('2027-02-01');
    expect(grid).toHaveLength(28);
    expect(grid.every(c => c.inMonth)).toBe(true);
  });

  it('has no repeated or skipped day in a month with a DST change', () => {
    const grid = monthGrid('2026-11-15');
    const dates = grid.map(c => c.date);
    expect(new Set(dates).size).toBe(dates.length);
    expect(dates[0]).toBe('2026-10-26');
    expect(dates.at(-1)).toBe('2026-12-06');
  });
});

describe('local time to UTC', () => {
  it('writes RFC 3339 without fractional seconds', () => {
    expect(toUtc(new Date('2026-10-13T07:30:00.123Z'))).toBe('2026-10-13T07:30:00Z');
  });

  it('converts wall-clock times using the offset of that day', () => {
    expect(toUtc(atLocal('2026-10-13', '09:30'))).toBe('2026-10-13T13:30:00Z'); // EDT, UTC-4
    expect(toUtc(atLocal('2026-12-01', '09:30'))).toBe('2026-12-01T14:30:00Z'); // EST, UTC-5
  });

  it('crosses the UTC date line late in the evening', () => {
    expect(toUtc(atLocal('2026-10-13', '23:00'))).toBe('2026-10-14T03:00:00Z');
    expect(toUtc(atLocal('2026-10-13', '00:00'))).toBe('2026-10-13T04:00:00Z');
  });

  it('bounds local days in UTC, including the 23-hour and 25-hour days', () => {
    expect(utcRange('2026-10-13', '2026-10-14')).toEqual(['2026-10-13T04:00:00Z', '2026-10-14T04:00:00Z']);
    expect(utcRange('2026-03-08', '2026-03-09')).toEqual(['2026-03-08T05:00:00Z', '2026-03-09T04:00:00Z']);
    expect(utcRange('2026-11-01', '2026-11-02')).toEqual(['2026-11-01T04:00:00Z', '2026-11-02T05:00:00Z']);
  });

  it('places minutes after midnight on the wall clock', () => {
    expect(hhmm(atMinutes('2026-10-13', 15 * 60 + 45))).toBe('15:45');
    // On the day clocks go back, 09:00 is still 09:00 although ten hours have passed since midnight.
    expect(hhmm(atMinutes('2026-11-01', 9 * 60))).toBe('09:00');
    expect(toUtc(atMinutes('2026-11-01', 9 * 60))).toBe('2026-11-01T14:00:00Z');
    expect(ymd(atMinutes('2026-10-13', 24 * 60))).toBe('2026-10-14');
  });
});

describe('the part of an interval on a day', () => {
  const at = (iso: string) => new Date(iso);

  it('returns wall-clock hours for an event within the day', () => {
    expect(spanOnDay(at('2026-10-13T14:00:00Z'), at('2026-10-13T15:30:00Z'), '2026-10-13')).toEqual({ s: 10, e: 11.5 });
  });

  it('is null on other days', () => {
    expect(spanOnDay(at('2026-10-13T14:00:00Z'), at('2026-10-13T15:30:00Z'), '2026-10-14')).toBeNull();
  });

  it('clips an event that crosses midnight to each day', () => {
    const start = atLocal('2026-10-13', '22:00');
    const end = atLocal('2026-10-14', '01:30');
    expect(spanOnDay(start, end, '2026-10-13')).toEqual({ s: 22, e: 24 });
    expect(spanOnDay(start, end, '2026-10-14')).toEqual({ s: 0, e: 1.5 });
  });

  it('does not spill an event that ends at midnight into the next day', () => {
    const start = atLocal('2026-10-13', '22:00');
    const end = atLocal('2026-10-14', '00:00');
    expect(spanOnDay(start, end, '2026-10-13')).toEqual({ s: 22, e: 24 });
    expect(spanOnDay(start, end, '2026-10-14')).toBeNull();
  });

  it('uses the wall clock after a DST change', () => {
    expect(spanOnDay(atLocal('2026-03-08', '10:00'), atLocal('2026-03-08', '11:00'), '2026-03-08')).toEqual({ s: 10, e: 11 });
    expect(spanOnDay(atLocal('2026-11-01', '10:00'), atLocal('2026-11-01', '11:00'), '2026-11-01')).toEqual({ s: 10, e: 11 });
  });

  it('keeps a zero-length entry on its day', () => {
    const moment = atLocal('2026-10-13', '08:00');
    expect(spanOnDay(moment, moment, '2026-10-13')).toEqual({ s: 8, e: 8 });
    expect(spanOnDay(moment, moment, '2026-10-12')).toBeNull();
  });
});

describe('snapping', () => {
  it('rounds to the nearest quarter of an hour', () => {
    expect(snap(7)).toBe(0);
    expect(snap(8)).toBe(15);
    expect(snap(52)).toBe(45);
    expect(snap(-8)).toBe(-15);
    expect(snap(-7)).toBe(-0);
  });
});
