// Item state shared by every list: optimistic changes, the open card, and the new-item draft.
import { signal } from '@preact/signals';
import * as api from '../api/client';
import type { Item, ItemWrite } from '../api/types';
import { onSettled } from './resource';
import { attempt, onExternalChange, onSessionEnd, refresh, reportError, version } from './store';

interface Patch {
  item: Item;
  /** The data version this change was made against; fresher data replaces it. */
  v: number;
}

const patches = signal<Record<string, Patch>>({});
const removed = signal<Record<string, number>>({});
/**
 * Items checked off (or back on) in the list on screen. They stay where they are although the
 * server no longer lists them there, and are let go when the list is left.
 */
export const held = signal<Record<string, Item>>({});

/** The item as it should be shown: the server's copy, or a newer local one. */
export const current = (item: Item): Item => patches.value[item.id]?.item ?? item;
export const isRemoved = (id: string): boolean => id in removed.value;

function setPatch(item: Item): void {
  patches.value = { ...patches.value, [item.id]: { item, v: version.value } };
  // Copies kept outside the fetched lists must not fall behind.
  if (item.id in held.value) held.value = { ...held.value, [item.id]: item };
  const d = draft.value;
  if (d?.item?.id === item.id) draft.value = { ...d, item };
}

function dropPatch(id: string): void {
  const { [id]: _, ...rest } = patches.value;
  patches.value = rest;
}

function hold(item: Item | null, id: string): void {
  const { [id]: _, ...rest } = held.value;
  held.value = item ? { ...rest, [id]: item } : rest;
}

// Once every list has refetched, local copies made before that refetch are no longer needed.
onSettled(() => {
  const keep = <T,>(all: Record<string, T>, v: (x: T) => number) =>
    Object.fromEntries(Object.entries(all).filter(([, x]) => v(x) >= version.value));
  patches.value = keep(patches.value, p => p.v);
  removed.value = keep(removed.value, n => n);
});
onExternalChange(() => { patches.value = {}; });

/** Applies a change at once, then confirms it with the server; rolls back if that fails. */
async function optimistic(item: Item, change: Partial<Item>, body: ItemWrite & { status?: Item['status'] }): Promise<Item | undefined> {
  setPatch({ ...item, ...change });
  try {
    const saved = await api.updateItem(item.id, body);
    setPatch(saved);
    return saved;
  } catch (err) {
    dropPatch(item.id);
    reportError(err);
    return undefined;
  } finally {
    void refresh();
  }
}

export async function toggleDone(item: Item): Promise<void> {
  const done = item.status !== 'done';
  const change: Partial<Item> = { status: done ? 'done' : 'open', completed_at: done ? new Date().toISOString() : null };
  hold({ ...item, ...change }, item.id);
  if (!(await optimistic(item, change, { status: change.status }))) hold(null, item.id);
}

export const toggleImportant = (item: Item) => optimistic(item, { important: !item.important }, { important: !item.important });

/** Saves a change made in the card and shows the server's copy of the item. */
export async function saveItem(work: Promise<Item>): Promise<Item | undefined> {
  const saved = await attempt(work);
  if (saved) setPatch(saved);
  void refresh();
  return saved;
}

export async function removeItem(id: string): Promise<boolean> {
  removed.value = { ...removed.value, [id]: version.value };
  try {
    await api.deleteItem(id);
    return true;
  } catch (err) {
    const { [id]: _, ...rest } = removed.value;
    removed.value = rest;
    reportError(err);
    return false;
  } finally {
    void refresh();
  }
}

/* ---------- the open card and the draft of a new item ---------- */

/** Id used by the card of an item that has not been created yet. */
export const NEW = '@new';

export interface Draft {
  /** Where the new item goes. */
  context: Pick<Item, 'project_id' | 'heading_id' | 'planned_date'>;
  /** Set once the item exists on the server; the card stays where it is. */
  item: Item | null;
}

/** Id of the item shown as a card, or NEW. */
export const openCard = signal<string | null>(null);
export const draft = signal<Draft | null>(null);

export function openItem(id: string | null): void {
  draft.value = null;
  openCard.value = id;
}

export function startDraft(context: Draft['context']): void {
  draft.value = { context, item: null };
  openCard.value = NEW;
}

/** Leaving a list closes its card and lets checked-off items be filed away. */
export function resetListState(): void {
  draft.value = null;
  openCard.value = null;
  held.value = {};
}

onSessionEnd(() => {
  resetListState();
  patches.value = {};
  removed.value = {};
});
