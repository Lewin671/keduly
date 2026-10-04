import { describe, expect, it } from 'vitest';
import type { CalEvent } from '../api/types';
import { allDayOn, eventSpan, onDay, stepDate, viewRange, weekTitle } from './calendar';
import { atLocal, toUtc } from './dates';

const event = (fields: Partial<CalEvent>): CalEvent => ({
  id: 'e', project_id: null, item_id: null, title: 't', notes: '', location: '', all_day: false, start: null, end: null,
  start_date: null, end_date: null, rrule: null, recurring: false, instance: null, status: 'confirmed', suggestion_id: null,
  item_done: null, readonly: false, created_by: { kind: 'user', name: 'Me' }, updated_at: '2026-10-01T00:00:00Z', ...fields,
});

describe('events on days', () => {
  it('lays a timed event on its local day', () => {
    const e = event({ start: toUtc(atLocal('2026-10-13', '10:00')), end: toUtc(atLocal('2026-10-13', '11:30')) });
    expect(eventSpan(e, '2026-10-13')).toEqual({ s: 10, e: 11.5 });
    expect(onDay(e, '2026-10-14')).toBe(false);
  });

  it('covers every day of an all-day event, both ends inclusive', () => {
    const e = event({ all_day: true, start_date: '2026-10-13', end_date: '2026-10-16' });
    expect(['2026-10-12', '2026-10-13', '2026-10-16', '2026-10-17'].map(d => allDayOn(e, d))).toEqual([false, true, true, false]);
    expect(eventSpan(e, '2026-10-14')).toBeNull();
  });
});

describe('paging', () => {
  it('steps by the size of the view', () => {
    expect(stepDate('day', '2026-10-31', 1)).toBe('2026-11-01');
    expect(stepDate('week', '2026-10-13', -1)).toBe('2026-10-06');
    expect(stepDate('month', '2026-10-31', 1)).toBe('2026-11-01');
    expect(stepDate('month', '2026-01-15', -1)).toBe('2025-12-01');
    expect(stepDate('year', '2026-10-13', 1)).toBe('2027-10-01');
  });

  it('loads the whole week for the day and week views, and its neighbours', () => {
    expect(viewRange('day', '2026-10-13')).toEqual({
      range: ['2026-10-12', '2026-10-19'],
      neighbours: [['2026-10-19', '2026-10-26'], ['2026-10-05', '2026-10-12']],
    });
    expect(viewRange('week', '2026-10-18').range).toEqual(['2026-10-12', '2026-10-19']);
  });

  it('loads the month for the month view, and its neighbours', () => {
    expect(viewRange('month', '2026-12-25')).toEqual({
      range: ['2026-12-01', '2027-01-01'],
      neighbours: [['2027-01-01', '2027-02-01'], ['2026-11-01', '2026-12-01']],
    });
  });

  it('titles a week by its Thursday', () => {
    expect(weekTitle('2026-10-13')).toEqual({ year: 2026, month: 10, week: 42 });
    // The week of Monday 2026-12-28 belongs to December and is week 53.
    expect(weekTitle('2027-01-02')).toEqual({ year: 2026, month: 12, week: 53 });
    // The week of Monday 2026-09-28 has its Thursday in October.
    expect(weekTitle('2026-09-28')).toEqual({ year: 2026, month: 10, week: 40 });
  });
});
