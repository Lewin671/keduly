// Calendar state that is not in the URL: the open popover, and writes made from the grid.
import { signal } from '@preact/signals';
import * as api from '../api/client';
import type { CalEvent } from '../api/types';
import { eventKey, patchEvents } from './events';
import { refresh, reportError } from './store';

/** Fields of the event editor. */
export interface EventDraft {
  title: string;
  allDay: boolean;
  date: string;
  endDate: string;
  start: string;
  end: string;
  projectId: string | null;
  notes: string;
}

export type CalPop =
  /** An existing entry: the editor, a time block's actions, or a suggestion, depending on the entry. */
  | { kind: 'event'; event: CalEvent; day: string | null }
  | { kind: 'new'; draft: EventDraft; day: string | null };

/** The open popover and the day column or cell it hangs from (`null`: the toolbar). */
export const calPop = signal<CalPop | null>(null);

let closedAt = 0;

export function closeCalPop(): void {
  if (calPop.value) closedAt = Date.now();
  calPop.value = null;
}

/** A click that just dismissed a popover should not also start a new event. */
export const justClosed = () => Date.now() - closedAt < 350;

/** Applies a change to a cached event at once and puts it back if the server refuses. */
export async function changeEvent(event: CalEvent, change: Partial<CalEvent>, work: () => Promise<unknown>): Promise<void> {
  const key = eventKey(event);
  const restore = patchEvents(other => (eventKey(other) === key ? { ...other, ...change } : other));
  try {
    await work();
  } catch (err) {
    restore();
    reportError(err);
  }
  void refresh();
}

export function moveEvent(event: CalEvent, start: string, end: string): Promise<void> {
  return changeEvent(event, { start, end }, () =>
    event.item_id ? api.scheduleItem(event.item_id, start, end) : api.updateEvent(event.id, { start, end }));
}

export function toggleBlockDone(event: CalEvent): Promise<void> {
  const done = !event.item_done;
  return changeEvent(event, { item_done: done }, () => api.updateItem(event.item_id!, { status: done ? 'done' : 'open' }));
}
