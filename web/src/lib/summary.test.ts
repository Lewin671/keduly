import { describe, expect, it } from 'vitest';
import { cap, dayLabel, dueLabel, dur, overdueDays, slotLabel, stamp, whenLabel } from './format';
import { todaySummary } from './summary';
import { atLocal } from './dates';

const base = { freeMinutes: 210, unplannedMinutes: 0, unplanned: 0, important: 0, workEnd: '18:00' };

describe('summary of today', () => {
  it('says everything is planned', () => {
    expect(todaySummary(base)).toBe('18:00 前还有 3.5 小时空闲，事项都已安排');
  });

  it('counts unplanned items that fit', () => {
    expect(todaySummary({ ...base, unplanned: 3, unplannedMinutes: 75 })).toBe('18:00 前还有 3.5 小时空闲，3 件事项未安排（1.3 小时）');
  });

  it('names what to keep when the day overflows and some items are important', () => {
    expect(todaySummary({ ...base, freeMinutes: 60, unplanned: 5, unplannedMinutes: 240, important: 2 }))
      .toBe('18:00 前还有 1 小时空闲，5 件事项未安排（4 小时），排不下：先保住 2 件重要的，其余 3 件可以往后放');
  });

  it('asks for a choice when nothing, or everything, is important', () => {
    expect(todaySummary({ ...base, freeMinutes: 30, unplanned: 2, unplannedMinutes: 90 }))
      .toBe('18:00 前还有 30 分钟空闲，2 件事项未安排（1.5 小时），排不下，需要取舍');
    expect(todaySummary({ ...base, freeMinutes: 30, unplanned: 2, unplannedMinutes: 90, important: 2 }))
      .toBe('18:00 前还有 30 分钟空闲，2 件事项未安排（1.5 小时），排不下，需要取舍');
  });

  it('says the working day is full, using the user’s own end of day', () => {
    expect(todaySummary({ ...base, freeMinutes: 0, workEnd: '17:30' })).toBe('17:30 前已经排满，事项都已安排');
    expect(todaySummary({ ...base, freeMinutes: 0, unplanned: 1, unplannedMinutes: 15 })).toBe('18:00 前已经排满，1 件事项未安排（15 分钟），排不下，需要取舍');
  });

  it('does not complain about unplanned items without estimates', () => {
    expect(todaySummary({ ...base, freeMinutes: 0, unplanned: 2, unplannedMinutes: 0 })).toBe('18:00 前已经排满，2 件事项未安排（0 分钟）');
  });
});

describe('wording of amounts and dates', () => {
  const today = '2026-10-13';

  it('words durations', () => {
    expect(dur(15)).toBe('15 分钟');
    expect(dur(60)).toBe('1 小时');
    expect(dur(90)).toBe('1.5 小时');
    expect(dur(120)).toBe('2 小时');
  });

  it('caps large counts', () => {
    expect(cap(99)).toBe('99');
    expect(cap(100)).toBe('99+');
  });

  it('names days relative to today', () => {
    expect(dayLabel('2026-10-13', today)).toBe('今天');
    expect(dayLabel('2026-10-14', today)).toBe('明天');
    expect(dayLabel('2026-10-12', today)).toBe('昨天');
    expect(dayLabel('2026-10-16', today)).toBe('周五');
    expect(dayLabel('2026-10-21', today)).toBe('10月21日');
    expect(dayLabel('2027-01-05', today)).toBe('2027年1月5日');
  });

  it('words where a time block sits', () => {
    expect(whenLabel(atLocal('2026-10-13', '15:30'), today)).toBe('15:30');
    expect(whenLabel(atLocal('2026-10-15', '13:00'), today)).toBe('周四 13:00');
    expect(whenLabel(atLocal('2026-10-21', '10:00'), today)).toBe('10月21日');
    expect(slotLabel(atLocal('2026-10-13', '14:00'), today)).toBe('今天 14:00');
  });

  it('words deadlines', () => {
    expect(dueLabel('2026-10-13', '18:00', today)).toBe('今天 18:00');
    expect(dueLabel('2026-10-15', null, today)).toBe('周四');
    expect(overdueDays('2026-10-09', today)).toBe(4);
    expect(overdueDays('2026-10-13', today)).toBe(0);
  });

  it('stamps activity in local time', () => {
    expect(stamp('2026-10-13T11:58:00Z', today)).toBe('今天 07:58');
    expect(stamp('2026-10-13T01:14:00Z', today)).toBe('昨天 21:14');
    expect(stamp('2026-10-12T00:30:00Z', today)).toBe('10月11日 20:30');
  });
});
