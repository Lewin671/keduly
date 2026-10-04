// The focus timer. The server owns it: sessions have a start and an end there, and this module only
// counts down between two answers and asks again when the running session should have ended.
import { batch, computed, effect, signal } from '@preact/signals';
import * as api from '../api/client';
import type { Focus, FocusSession, Item } from '../api/types';
import { t } from '../i18n';
import { minutesSpent, secondsLeft } from '../lib/focus';
import { span } from '../lib/format';
import { readStored, writeStored } from '../lib/storage';
import { current } from './items';
import { navigate, route } from './route';
import { attempt, colorOf, onSessionEnd, refresh, reportError, session, today, version } from './store';
import { showHud } from './ui';

/** `null` until the first answer arrives. */
export const focus = signal<Focus | null>(null);
/** This device's clock, moved on while a timer runs so that countdowns redraw. */
export const clock = signal(Date.now());
/** The server's clock minus this device's, so the countdown is right when the device's clock is off. */
let skew = 0;
/** The server's time now, in milliseconds. Reading it redraws the reader as the clock moves. */
export const serverNow = (): number => clock.value + skew;
/** The same, for readers that should not redraw with every tick. */
export const serverTime = (): number => Date.now() + skew;

/** The item chosen for the next tomato: an id, `null` for free focus, `undefined` to take the first of today. */
export const pick = signal<string | null | undefined>(undefined);
/** Items a tomato was started on from outside Today, so the timer page can still describe them. */
const remembered = signal<Record<string, Item>>({});
/** Today's items, loaded while focus mode is showing. */
export const todayItems = signal<Item[] | undefined>(undefined);

/** The timer page takes the whole window while a timer is active. */
export const zen = computed(() => route.value.mode === 'focus' && route.value.focus === 'timer' && focus.value !== null && focus.value.state !== 'idle');

export const freeTitle = (): string => t('focus.free');
export const sessionTitle = (s: FocusSession): string => (s.item_id ? s.title : freeTitle());
/** The colour a timer takes: green while resting, otherwise its item's project. */
export const focusColor = (f: Focus): string => (f.state === 'rest' ? 'var(--green)' : colorOf(f.session?.project_id));
/** Whether a tomato is running on this item right now. */
export const workingOn = (itemId: string): boolean => focus.value?.state === 'work' && focus.value.session!.item_id === itemId;

function apply(next: Focus): void {
  skew = Date.parse(next.now) - Date.now();
  batch(() => {
    clock.value = Date.now();
    focus.value = next;
  });
}

export async function loadFocus(): Promise<void> {
  try {
    apply(await api.getFocus());
  } catch (err) {
    reportError(err);
  }
}

// Every load of the shared data, and every change noticed by the poll, reloads the timer too.
effect(() => {
  void version.value;
  if (session.peek() === 'ready') void loadFocus();
});

effect(() => {
  void version.value;
  void today.value;
  if (route.value.mode !== 'focus' || session.peek() !== 'ready') return;
  api.getToday().then(view => { todayItems.value = view.items; }, reportError);
});

onSessionEnd(() => {
  focus.value = null;
  pick.value = undefined;
  remembered.value = {};
  todayItems.value = undefined;
});

/* ---------- what the next tomato is for ---------- */

/** Today's open items, important first, with the picked item added when it is not one of them. */
export function picks(): Item[] {
  const list = (todayItems.value ?? []).map(current).filter(i => i.status === 'open').sort((a, b) => Number(b.important) - Number(a.important));
  const id = pick.value;
  const extra = typeof id === 'string' && !list.some(i => i.id === id) ? remembered.value[id] : undefined;
  if (extra && current(extra).status === 'open') list.unshift(current(extra));
  return list;
}

/** The item the next tomato goes to; `null` is free focus. */
export function pickedItem(): Item | null {
  if (pick.value === null) return null;
  const list = picks();
  return list.find(i => i.id === pick.value) ?? list[0] ?? null;
}

/** An item this module has seen, for the estimate and tomato count under the dial. */
export function knownItem(id: string | null): Item | undefined {
  if (!id) return undefined;
  const found = todayItems.value?.find(i => i.id === id) ?? remembered.value[id];
  return found && current(found);
}

/* ---------- the reminder, a preference of this browser ---------- */

const REMIND_KEY = 'keduly.focus.remind';
export const remind = signal(readStored(REMIND_KEY) !== 'off');

const canNotify = (): boolean => typeof Notification !== 'undefined';
export const notifyBlocked = (): boolean => !canNotify() || Notification.permission === 'denied';

function askToNotify(): void {
  if (remind.value && canNotify() && Notification.permission === 'default') void Notification.requestPermission();
}

export function setRemind(on: boolean): void {
  remind.value = on;
  writeStored(REMIND_KEY, on ? null : 'off');
  askToNotify();
}

function beep(): void {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    const t0 = ctx.currentTime;
    osc.frequency.value = 880;
    gain.gain.setValueAtTime(0.001, t0);
    gain.gain.exponentialRampToValueAtTime(0.2, t0 + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.001, t0 + 0.6);
    osc.connect(gain).connect(ctx.destination);
    osc.start();
    osc.stop(t0 + 0.65);
    osc.onended = () => { void ctx.close(); };
  } catch {
    // No sound on this device, or the browser has not allowed it yet.
  }
}

function notify(text: string): void {
  if (!remind.value) return;
  beep();
  try {
    if (canNotify() && Notification.permission === 'granted') new Notification('Keduly', { body: text });
  } catch {
    // Some browsers only allow notifications from a service worker.
  }
}

/* ---------- running out ---------- */

let asking = false;
let quietUntil = 0;

/**
 * Moves the countdown on. When the running session should have ended, the server is asked what
 * happened: it alone decides that a tomato was earned.
 */
export async function tickFocus(): Promise<void> {
  const before = focus.value;
  if (!before || (before.state !== 'work' && before.state !== 'rest')) return;
  clock.value = Date.now();
  if (asking || Date.now() < quietUntil || secondsLeft(before.session!, serverTime()) > 0) return;
  asking = true;
  try {
    const next = await api.getFocus();
    apply(next);
    // The server's clock is a moment behind: ask again on the next tick.
    if (next.state === before.state && next.session!.id === before.session!.id) return;
    if (before.state === 'work' && next.state === 'over') {
      pick.value = next.session!.item_id;
      const text = t('focus.earned', { n: next.tomatoes_today });
      notify(text);
      if (!zen.value) showHud(text, () => { void restFocus(); }, t('focus.startRest'));
    } else if (before.state === 'rest' && next.state === 'idle') {
      notify(t('focus.restOver'));
      showHud(t('focus.restOver'));
    }
    void refresh();
  } catch (err) {
    quietUntil = Date.now() + 5000;
    reportError(err);
  } finally {
    asking = false;
  }
}

export function startFocusClock(): void {
  setInterval(() => { void tickFocus(); }, 500);
}

/* ---------- actions ---------- */

export function openTimer(): void {
  navigate({ mode: 'focus', focus: 'timer' });
  scrollTo(0, 0);
}

/** Starts a tomato on the item (`null`: free focus) and shows the timer page. */
export async function startFocus(itemId: string | null, item?: Item): Promise<void> {
  if (item) remembered.value = { ...remembered.value, [item.id]: item };
  askToNotify();
  const before = focus.value;
  const spent = before?.state === 'work' ? minutesSpent(before.session!, serverTime()) : 0;
  const next = await attempt(api.startFocus(itemId));
  if (!next) return;
  apply(next);
  pick.value = itemId;
  openTimer();
  // Starting on something else gives up what was running.
  if (spent >= 1) showHud(t('focus.gaveUpOther', { title: sessionTitle(before!.session!), dur: span(spent) }));
  void refresh();
}

/** Gives up the running tomato, skips the rest, or leaves a finished tomato unanswered. */
export async function stopFocus(): Promise<void> {
  const before = focus.value;
  if (!before || before.state === 'idle') return;
  const spent = minutesSpent(before.session!, serverTime());
  const next = await attempt(api.stopFocus());
  if (!next) return;
  apply(next);
  if (before.state !== 'rest') pick.value = before.session!.item_id;
  if (before.state === 'work') showHud(spent >= 1 ? t('focus.gaveUp', { dur: span(spent) }) : t('focus.tooShort'));
  void refresh();
}

export async function restFocus(): Promise<void> {
  const next = await attempt(api.restFocus());
  if (next) apply(next);
  void refresh();
}

/** "Done": ticks the item the timer is on. The server ends the timer with it. */
export async function finishFocus(): Promise<void> {
  const s = focus.value!.session!;
  const saved = await attempt(api.updateItem(s.item_id!, { status: 'done' }));
  if (!saved) return;
  pick.value = undefined;
  showHud(t('focus.finished', { title: s.title }));
  await loadFocus();
  void refresh();
}

/** One more tomato on the same thing. */
export const againFocus = (): Promise<void> => startFocus(focus.value!.session!.item_id);
