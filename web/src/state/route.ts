// Mode, calendar view, current date and selected list live in the URL hash, so reload and
// back/forward work: #/cal/week/2026-10-13, #/items/today, #/items/p/<project id>.
import { signal } from '@preact/signals';
import type { CalView } from '../lib/calendar';
import { isYmd, ymd } from '../lib/dates';

export type Mode = 'cal' | 'items';

export interface Route {
  mode: Mode;
  view: CalView;
  date: string;
  /** A built-in list, or `p:<project id>`. */
  list: string;
}

const VIEWS: readonly string[] = ['day', 'week', 'month', 'year'];
const LISTS: readonly string[] = ['inbox', 'today', 'upcoming', 'matrix', 'all', 'done'];

/** Reads a hash. The mode that is not in the URL keeps the state it had. */
export function parseRoute(hash: string, previous: Route): Route {
  const [mode, a, b] = hash.replace(/^#\/?/, '').split('/');
  if (mode === 'items') {
    if (a === 'p' && b) return { ...previous, mode: 'items', list: `p:${decodeURIComponent(b)}` };
    return { ...previous, mode: 'items', list: a && LISTS.includes(a) ? a : 'today' };
  }
  if (mode === 'cal') {
    return {
      ...previous,
      mode: 'cal',
      view: a && VIEWS.includes(a) ? (a as CalView) : previous.view,
      date: b && isYmd(b) ? b : ymd(new Date()),
    };
  }
  return previous;
}

export function formatRoute(route: Route): string {
  if (route.mode === 'cal') return `#/cal/${route.view}/${route.date}`;
  return route.list.startsWith('p:') ? `#/items/p/${encodeURIComponent(route.list.slice(2))}` : `#/items/${route.list}`;
}

const initial: Route = { mode: 'cal', view: 'day', date: ymd(new Date()), list: 'today' };

export const route = signal<Route>(parseRoute(location.hash, initial));

export function navigate(change: Partial<Route>): void {
  const next = { ...route.value, ...change };
  const hash = formatRoute(next);
  if (hash === location.hash) return;
  route.value = next;
  history.pushState(null, '', hash);
}

export function initRoute(): void {
  history.replaceState(null, '', formatRoute(route.value));
  addEventListener('popstate', () => { route.value = parseRoute(location.hash, route.value); });
  addEventListener('hashchange', () => { route.value = parseRoute(location.hash, route.value); });
}
