// Mode, calendar view, current date and selected list live in the URL hash, so reload and
// back/forward work: #/cal/week/2026-10-13, #/items/today, #/items/p/<project id>, #/focus/stats.
import { signal } from '@preact/signals';
import type { CalView } from '../lib/calendar';
import { isYmd, ymd, wallNow } from '../lib/dates';

export type Mode = 'cal' | 'items' | 'focus';
export type FocusView = 'timer' | 'stats';

export interface Route {
  mode: Mode;
  view: CalView;
  date: string;
  /** A built-in list, or `p:<project id>`. */
  list: string;
  focus: FocusView;
}

const VIEWS: readonly string[] = ['day', 'week', 'month', 'year'];
const LISTS: readonly string[] = ['matrix', 'today', 'upcoming', 'all', 'inbox', 'done'];
/** The list the app opens on, and falls back to. */
export const HOME_LIST = 'matrix';

/** Reads a hash. The mode that is not in the URL keeps the state it had. */
export function parseRoute(hash: string, previous: Route): Route {
  const [mode, a, b] = hash.replace(/^#\/?/, '').split('/');
  if (mode === 'items') {
    if (a === 'p' && b) return { ...previous, mode: 'items', list: `p:${decodeURIComponent(b)}` };
    return { ...previous, mode: 'items', list: a && LISTS.includes(a) ? a : HOME_LIST };
  }
  if (mode === 'cal') {
    return {
      ...previous,
      mode: 'cal',
      view: a && VIEWS.includes(a) ? (a as CalView) : previous.view,
      date: b && isYmd(b) ? b : ymd(wallNow()),
    };
  }
  if (mode === 'focus') return { ...previous, mode: 'focus', focus: a === 'stats' ? 'stats' : 'timer' };
  return previous;
}

export function formatRoute(route: Route): string {
  if (route.mode === 'cal') return `#/cal/${route.view}/${route.date}`;
  if (route.mode === 'focus') return `#/focus/${route.focus}`;
  return route.list.startsWith('p:') ? `#/items/p/${encodeURIComponent(route.list.slice(2))}` : `#/items/${route.list}`;
}

// The app opens on the matrix, which holds every open item: managing things to do is the product's centre, the calendar a second view.
const initial: Route = { mode: 'items', view: 'day', date: ymd(wallNow()), list: HOME_LIST, focus: 'timer' };

export const route = signal<Route>(parseRoute(location.hash, initial));

let behind: Exclude<Mode, 'focus'> = 'items';
/** The mode that was showing before focus, which is where its timer page collapses back to. */
export const modeBehindFocus = (): Exclude<Mode, 'focus'> => behind;

function show(next: Route): void {
  if (next.mode !== 'focus') behind = next.mode;
  route.value = next;
}

export function navigate(change: Partial<Route>): void {
  const next = { ...route.value, ...change };
  const hash = formatRoute(next);
  if (hash === location.hash) return;
  show(next);
  history.pushState(null, '', hash);
}

export function initRoute(): void {
  history.replaceState(null, '', formatRoute(route.value));
  addEventListener('popstate', () => show(parseRoute(location.hash, route.value)));
  addEventListener('hashchange', () => show(parseRoute(location.hash, route.value)));
}
