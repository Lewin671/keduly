// The sentence at the top of Today: how much free time is left and whether the unplanned items fit.
import { t } from '../i18n';
import { dur } from './format';

export interface TodayBudget {
  /** Free minutes within working hours. */
  freeMinutes: number;
  /** Sum of the estimates of the unplanned items. */
  unplannedMinutes: number;
  /** Open daytime items with neither a time block nor a pending suggestion. */
  unplanned: number;
  /** How many of those are marked important. */
  important: number;
  /** End of the working day, `HH:MM`. */
  workEnd: string;
}

export function todaySummary(b: TodayBudget): string {
  const free = b.freeMinutes <= 0 ? t('today.full', { end: b.workEnd }) : t('today.free', { end: b.workEnd, dur: dur(b.freeMinutes) });
  if (!b.unplanned) return free + t('today.allPlanned');
  const rest = b.unplanned - b.important;
  const verdict =
    b.unplannedMinutes <= b.freeMinutes ? ''
    : b.important && rest ? t('today.triage', { keep: b.important, rest })
    : t('today.overflow');
  return free + t('today.unplanned', { n: b.unplanned, dur: dur(b.unplannedMinutes) }) + verdict;
}
