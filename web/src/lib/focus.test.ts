import { describe, expect, it } from 'vitest';
import type { FocusSession } from '../api/types';
import { span } from './format';
import { chartScale, fractionLeft, minutesSpent, mmss, secondsLeft, sessionsByDay, tomatoesNeeded } from './focus';

const session = (id: string, start: string, end: string, completed = true): FocusSession => ({
  id, kind: 'work', item_id: null, project_id: null, title: '', start, end, planned_minutes: 25, completed, created_by: { kind: 'user', name: 'Me' },
});

describe('countdown', () => {
  it('shows a second that has begun, and never goes below zero', () => {
    expect(mmss(25 * 60)).toBe('25:00');
    expect(mmss(59.2)).toBe('01:00');
    expect(mmss(0.4)).toBe('00:01');
    expect(mmss(-3)).toBe('00:00');
  });

  it('measures what is left against the server clock', () => {
    const s = session('a', '2026-10-13T02:00:00Z', '2026-10-13T02:25:00Z');
    const at = (time: string) => Date.parse(`2026-10-13T${time}Z`);
    expect(secondsLeft(s, at('02:10:00'))).toBe(900);
    expect(secondsLeft(s, at('03:00:00'))).toBe(0);
    expect(fractionLeft(s, at('02:00:00'))).toBe(1);
    expect(fractionLeft(s, at('02:12:30'))).toBe(0.5);
    expect(minutesSpent(s, at('02:10:00'))).toBe(10);
    expect(minutesSpent(s, at('04:00:00'))).toBe(25);
  });
});

describe('tomatoes and time', () => {
  it('turns an estimate into tomatoes, rounding up', () => {
    expect(tomatoesNeeded(90, 25)).toBe(4);
    expect(tomatoesNeeded(30, 25)).toBe(2);
    expect(tomatoesNeeded(25, 25)).toBe(1);
    expect(tomatoesNeeded(15, 25)).toBe(1);
    expect(tomatoesNeeded(null, 25)).toBe(0);
  });

  it('words time spent to the minute', () => {
    expect(span(50)).toBe('50 分钟');
    expect(span(120)).toBe('2 小时');
    expect(span(135)).toBe('2 小时 15 分钟');
    expect(span(0.3)).toBe('1 分钟');
  });

  it('scales the chart to an even number of at least 4', () => {
    expect(chartScale([{ tomatoes: 0 }])).toBe(4);
    expect(chartScale([{ tomatoes: 5 }, { tomatoes: 2 }])).toBe(6);
    expect(chartScale([{ tomatoes: 14 }])).toBe(14);
    expect(chartScale([])).toBe(4);
  });
});

describe('records', () => {
  it('groups ended sessions by day, newest first, and leaves out the one still running', () => {
    const now = Date.parse('2026-10-13T12:00:00');
    const iso = (local: string) => new Date(local).toISOString();
    const days = sessionsByDay([
      session('a', iso('2026-10-12T10:00:00'), iso('2026-10-12T10:25:00')),
      session('b', iso('2026-10-13T09:00:00'), iso('2026-10-13T09:25:00')),
      session('c', iso('2026-10-13T10:00:00'), iso('2026-10-13T10:12:00'), false),
      session('d', iso('2026-10-13T11:50:00'), iso('2026-10-13T12:15:00'), false),
    ], now);
    expect(days.map(d => d.date)).toEqual(['2026-10-13', '2026-10-12']);
    expect(days[0]!.sessions.map(s => s.id)).toEqual(['c', 'b']);
  });
});
