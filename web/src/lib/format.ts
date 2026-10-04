// How dates, times and amounts are worded in the interface.
import { t } from '../i18n';
import { addDays, daysBetween, hhmm, mondayIndex, startOfWeek, ymd, wall } from './dates';

/** A length of time: "45 分钟", "2 小时", "1.5 小时". */
export function dur(minutes: number): string {
  if (minutes < 60) return t('dur.minutes', { n: Math.round(minutes) });
  const hours = minutes / 60;
  return t('dur.hours', { n: Number.isInteger(hours) ? hours : hours.toFixed(1) });
}

/** Counts above 99 are not spelled out. */
export function cap(n: number): string {
  return n > 99 ? '99+' : String(n);
}

export function weekdayShort(day: string): string {
  return t('day.short').split(',')[mondayIndex(day)]!;
}

export function weekdayLong(day: string): string {
  return t('day.long').split(',')[mondayIndex(day)]!;
}

export function monthDay(day: string, today: string): string {
  const [y, m, d] = day.split('-').map(Number);
  return y === Number(today.slice(0, 4)) ? t('date.md', { m: m!, d: d! }) : t('date.ymd', { y: y!, m: m!, d: d! });
}

/** "今天", "明天", the weekday within the current week, otherwise the date. */
export function dayLabel(day: string, today: string): string {
  if (day === today) return t('day.today');
  if (day === addDays(today, 1)) return t('day.tomorrow');
  if (day === addDays(today, -1)) return t('day.yesterday');
  if (startOfWeek(day) === startOfWeek(today)) return weekdayShort(day);
  return monthDay(day, today);
}

/** Where a time block sits, as short as a list row needs: "15:30", "周三 14:00", "10月21日". */
export function whenLabel(start: Date, today: string): string {
  const day = ymd(start);
  if (day === today) return hhmm(start);
  if (startOfWeek(day) === startOfWeek(today)) return `${weekdayShort(day)} ${hhmm(start)}`;
  return monthDay(day, today);
}

/** A day and a time in full: "今天 14:00", "周三 10:30", "10月21日 10:00". */
export function slotLabel(start: Date, today: string): string {
  return `${dayLabel(ymd(start), today)} ${hhmm(start)}`;
}

export function dueLabel(dueDate: string, dueTime: string | null, today: string): string {
  const day = dayLabel(dueDate, today);
  return dueTime ? `${day} ${dueTime}` : day;
}

export function overdueDays(dueDate: string, today: string): number {
  return daysBetween(dueDate, today);
}

/** When something happened, for the activity log: "今天 07:58", "昨天 21:14", "10月11日 20:30". */
export function stamp(iso: string, today: string): string {
  const date = wall(iso);
  const day = ymd(date);
  const label = day === today ? t('day.today') : day === addDays(today, -1) ? t('day.yesterday') : monthDay(day, today);
  return `${label} ${hhmm(date)}`;
}
