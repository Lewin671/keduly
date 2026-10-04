// Session, the data every screen shares, and change detection.
import { batch, computed, signal } from '@preact/signals';
import * as api from '../api/client';
import { ApiError } from '../api/client';
import type { Area, Bootstrap, Config, Counts, Heading, Project, Suggestion, User } from '../api/types';
import { hasMessage, t } from '../i18n';
import { ymd, wallNow, deviceZone, setZone } from '../lib/dates';
import { readStored, writeStored } from '../lib/storage';
import { showToast } from './ui';

export const session = signal<'loading' | 'anon' | 'ready'>('loading');
export const config = signal<Config | null>(null);
export const user = signal<User | null>(null);
export const areas = signal<Area[]>([]);
export const projects = signal<Project[]>([]);
export const headings = signal<Heading[]>([]);
export const counts = signal<Counts>({ inbox: 0, today: 0, pending: 0 });
/** Pending suggestions, for the bell panel and the calendar popovers. */
export const suggestions = signal<Suggestion[]>([]);
/** Increases whenever what is on screen may be stale; data hooks refetch when it changes. */
export const version = signal(0);

export const now = signal(wallNow());
export const today = computed(() => ymd(now.value));

const ZONE_KEPT = 'keduly.zone.kept';
/** The device zone the user already declined to switch to, on this device. */
const zoneKept = signal(readStored(ZONE_KEPT));
/** The device's zone when it differs from the account's and the user has not answered yet. */
export const zoneQuestion = computed(() => {
  const me = user.value, device = deviceZone();
  return me && !me.timezone_auto && device && device !== me.timezone && device !== zoneKept.value ? device : null;
});
/** Stop asking on this device for as long as it stays in the zone it is in now. */
export function keepZone(): void {
  writeStored(ZONE_KEPT, deviceZone());
  zoneKept.value = deviceZone();
}

/** Projects that are not archived, in sidebar order. */
export const activeProjects = computed(() => projects.value.filter(p => !p.archived).sort((a, b) => a.position - b.position));

let revision = -1;
/** Callbacks run when the data changed somewhere else, before screens refetch. */
const externalChange = new Set<() => void>();
export const onExternalChange = (fn: () => void) => { externalChange.add(fn); };
/** Callbacks run when the session ends, so nothing of one account is shown to the next. */
const sessionEnd = new Set<() => void>();
export const onSessionEnd = (fn: () => void) => { sessionEnd.add(fn); };

/* ---------- errors ---------- */

export function errorText(err: unknown): string {
  const key = err instanceof ApiError ? `error.${err.code}` : '';
  return t(hasMessage(key) ? key : 'error.unknown');
}

/** Describes a failed request to the user; a lost session returns to the sign-in screen. */
export function reportError(err: unknown): void {
  if (err instanceof ApiError && err.status === 401) {
    if (session.value === 'ready') signOutLocally();
    return;
  }
  if (!(err instanceof ApiError)) console.error(err);
  showToast(errorText(err));
}

/** Awaits a request; on failure tells the user and resolves to `undefined`. */
export async function attempt<T>(work: Promise<T>): Promise<T | undefined> {
  try {
    return await work;
  } catch (err) {
    reportError(err);
    return undefined;
  }
}

/* ---------- loading ---------- */

function applyBootstrap(b: Bootstrap): void {
  // Times are shown in the account's zone, which is also the zone the server computes "today" in.
  setZone(b.user.timezone);
  batch(() => {
    user.value = b.user;
    now.value = wallNow();
    areas.value = [...b.areas].sort((x, y) => x.position - y.position);
    projects.value = b.projects;
    headings.value = [...b.headings].sort((x, y) => x.position - y.position);
    counts.value = b.counts;
    session.value = 'ready';
  });
  revision = b.revision;
}

async function loadSuggestions(): Promise<void> {
  suggestions.value = counts.value.pending > 0 ? await api.listSuggestions('pending') : [];
}

function signOutLocally(): void {
  batch(() => {
    session.value = 'anon';
    user.value = null;
    areas.value = [];
    projects.value = [];
    headings.value = [];
    suggestions.value = [];
    counts.value = { inbox: 0, today: 0, pending: 0 };
  });
  revision = -1;
  for (const fn of sessionEnd) fn();
}

/** First load: is registration open, and is there a session? */
export async function boot(): Promise<void> {
  api.getConfig().then(c => { config.value = c; }, () => { /* sign-up stays hidden */ });
  await enter();
}

/** Loads the signed-in user's data, or shows the sign-in screen. */
export async function enter(): Promise<void> {
  try {
    let boot = await api.getBootstrap();
    // Only when the user opted in does the account follow the device silently; otherwise
    // `zoneQuestion` asks.
    const device = deviceZone();
    if (boot.user.timezone_auto && device && boot.user.timezone !== device) {
      await api.updateMe({ timezone: device });
      boot = await api.getBootstrap();
    }
    applyBootstrap(boot);
    version.value++;
    await loadSuggestions();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) signOutLocally();
    else {
      session.value = 'anon';
      reportError(err);
    }
  }
}

/** Loads the account this device was just signed in to, dropping what another one left on screen. */
export async function enterFresh(): Promise<void> {
  if (session.value === 'ready') signOutLocally();
  await enter();
}

export async function signOut(): Promise<void> {
  await attempt(api.logout());
  signOutLocally();
}

/** Reloads the shared data and makes every screen refetch. Call after each write. */
export async function refresh(): Promise<void> {
  try {
    applyBootstrap(await api.getBootstrap());
    version.value++;
    await loadSuggestions();
  } catch (err) {
    reportError(err);
  }
}

/** Runs a write, reports its failure, and refreshes afterwards either way. */
export async function write<T>(work: Promise<T>): Promise<T | undefined> {
  const result = await attempt(work);
  void refresh();
  return result;
}

/** Asks the server whether anything changed since the last load. */
export async function poll(): Promise<void> {
  if (session.value !== 'ready' || document.hidden) return;
  try {
    const res = await api.getCounts();
    counts.value = res.counts;
    if (res.revision !== revision) {
      for (const fn of externalChange) fn();
      await refresh();
    }
  } catch (err) {
    // A failed poll is only worth reporting when it means the session ended.
    if (err instanceof ApiError && err.status === 401) reportError(err);
  }
}

const POLL_MS = 20_000;

export function startClock(): void {
  setInterval(() => { void poll(); }, POLL_MS);
  addEventListener('focus', () => { void poll(); });
  setInterval(() => { now.value = wallNow(); }, 30_000);
}

/* ---------- lookups ---------- */

export function projectOf(id: string | null | undefined): Project | undefined {
  return id ? projects.value.find(p => p.id === id) : undefined;
}

/** Events and items take their project's colour; without one they are grey. */
export function colorOf(projectId: string | null | undefined): string {
  const project = projectOf(projectId);
  return project ? `var(--${project.color})` : 'var(--gray)';
}

/* ---------- projects hidden in the calendar, per browser ---------- */

const HIDDEN_KEY = 'keduly.hiddenProjects';

function readHidden(): Set<string> {
  try {
    const list: unknown = JSON.parse(readStored(HIDDEN_KEY) ?? '[]');
    return new Set(Array.isArray(list) ? list.filter((x): x is string => typeof x === 'string') : []);
  } catch {
    return new Set();
  }
}

export const hiddenProjects = signal<Set<string>>(readHidden());

export function toggleProjectVisible(id: string): void {
  const next = new Set(hiddenProjects.value);
  if (!next.delete(id)) next.add(id);
  hiddenProjects.value = next;
  writeStored(HIDDEN_KEY, JSON.stringify([...next]));
}
