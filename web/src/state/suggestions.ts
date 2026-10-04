// Deciding suggestions: accepting can be undone through the activity it creates.
import * as api from '../api/client';
import type { Suggestion } from '../api/types';
import { t } from '../i18n';
import { hhmm, wall } from '../lib/dates';
import { slotLabel } from '../lib/format';
import { attempt, refresh, suggestions, today, write } from './store';
import { showHud } from './ui';

export async function decide(id: string, title: string, accept: boolean): Promise<void> {
  if (accept) {
    const res = await write(api.acceptSuggestion(id));
    if (res) showHud(t('hud.accepted', { title }), () => { void write(api.undoActivity(res.activity.id)); });
  } else {
    const res = await write(api.rejectSuggestion(id));
    if (res) showHud(t('hud.rejected', { title }));
  }
}

const isDeletion = (s: Suggestion) => s.kind === 'delete_event' || s.kind === 'delete_item';

/** Pending suggestions that would delete something: each needs its own yes or no. */
export const pendingDeletions = () => suggestions.value.filter(isDeletion);
/** Pending suggestions that schedule, create or move. */
export const pendingPlans = () => suggestions.value.filter(s => !isDeletion(s));

/**
 * Accepts every pending scheduling suggestion. Deletions are never swept up by this: while one
 * is waiting, the others are accepted one by one instead of through the accept-all endpoint.
 */
export async function acceptAllPlans(): Promise<void> {
  const plans = pendingPlans();
  let ids: string[] = [];
  if (pendingDeletions().length) {
    for (const s of plans) {
      const res = await attempt(api.acceptSuggestion(s.id));
      if (res) ids.push(res.activity.id);
    }
  } else {
    ids = (await attempt(api.acceptAllSuggestions()))?.activity_ids ?? [];
  }
  void refresh();
  if (ids.length) showHud(t('hud.acceptedAll', { n: ids.length }), () => { void write(api.undoActivities(ids)); });
}

/** What a suggestion would do, in a few words. */
export function describe(s: Pick<Suggestion, 'kind' | 'start' | 'end' | 'event'>): string {
  const slot = s.start ? slotLabel(wall(s.start), today.value) : '';
  switch (s.kind) {
    case 'schedule_item': return t('suggest.schedule', { slot });
    case 'create_item': return slot ? t('suggest.createItemAt', { slot }) : t('suggest.createItem');
    case 'create_event': return t('suggest.createEvent', { slot });
    case 'move_event': {
      const from = s.event?.start ? wall(s.event.start) : null;
      return from ? t('suggest.moveFrom', { from: slotLabel(from, today.value), to: slot }) : t('suggest.move', { slot });
    }
    case 'delete_event': return t('suggest.deleteEvent');
    case 'delete_item': return t('suggest.deleteItem');
  }
}

export const timeRange = (start: string, end: string) => `${hhmm(wall(start))}–${hhmm(wall(end))}`;
