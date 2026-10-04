// Pieces every list is built from.
import type { ComponentChildren, JSX } from 'preact';
import { useRef } from 'preact/hooks';
import type { CalEvent, Item } from '../../api/types';
import { t } from '../../i18n';
import { hhmm, wall } from '../../lib/dates';
import { keepInPlace } from '../../lib/sticky';
import { draft, held, isRemoved } from '../../state/items';
import { colorOf } from '../../state/store';
import { ItemRow, type RowOptions } from './ItemRow';

export function ListHead({ icon, name, color, children }: { icon: JSX.Element; name: ComponentChildren; color: string; children?: ComponentChildren }): JSX.Element {
  return (
    <div class="lh" style={{ '--c': color }}>
      {icon}
      <h2>{name}</h2>
      {children}
    </div>
  );
}

export function Blank({ title, tip }: { title: string; tip: string }): JSX.Element {
  return <div class="blank"><b>{title}</b><span>{tip}</span></div>;
}

const WIDTHS = [46, 30, 38];

/** Placeholder rows shown while a batch is on its way. */
export function Skeleton({ rows = 3, pad = false }: { rows?: number; pad?: boolean }): JSX.Element {
  return (
    <div aria-busy="true" aria-label={t('common.loading')}>
      {Array.from({ length: rows }, (_, k) => (
        <div class={pad ? 'sk pad' : 'sk'}><i /><b style={{ width: `${WIDTHS[k % 3]}%` }} /></div>
      ))}
    </div>
  );
}

export function LoadFailed({ retry }: { retry: () => void }): JSX.Element {
  return (
    <div class="blank">
      <b>{t('error.load')}</b>
      <button class="tb" onClick={retry}>{t('common.retry')}</button>
    </div>
  );
}

/**
 * Rows of a list. An item checked off here stays where it was until the list is left, and the
 * item open as the new-item card is not listed a second time.
 */
export function Rows({ items, opts }: { items: readonly Item[]; opts?: RowOptions }): JSX.Element {
  const previous = useRef<readonly Item[]>([]);
  const kept = held.value;
  const shown = keepInPlace(previous.current, items, id => kept[id]);
  previous.current = shown;
  const drafted = draft.value?.item?.id;
  return (
    <>
      {shown.filter(item => item.id !== drafted && !isRemoved(item.id)).map(item => <ItemRow key={item.id} item={item} opts={opts} />)}
    </>
  );
}

/** How many more there are, and the button that fetches the next batch. */
export function MoreButton({ rest, step, loading, onMore }: { rest: number; step: number; loading: boolean; onMore: () => void }): JSX.Element | null {
  if (loading) return <Skeleton />;
  if (rest <= 0) return null;
  return (
    <button class="morebtn" onClick={onMore}>
      {rest > step ? t('list.more', { step, rest }) : t('list.rest', { rest })}
    </button>
  );
}

interface EventsProps {
  events: readonly CalEvent[];
  /** Above this many, the list is cut and the rest is left to the calendar. */
  max?: number;
  label?: (event: CalEvent) => string;
}

export const eventTime = (event: CalEvent) => (event.all_day || !event.start ? t('cal.allDay') : hhmm(wall(event.start)));

/** A day's calendar events in small grey type, each with its project's colour bar. */
export function EventsStrip({ events, max = 6, label = eventTime }: EventsProps): JSX.Element | null {
  if (!events.length) return null;
  const shown = events.length > max ? events.slice(0, max - 1) : events;
  return (
    <div class="evs">
      {shown.map(event => (
        <div class="ev" style={{ '--c': colorOf(event.project_id) }}>
          <time>{label(event)}</time>
          <span title={event.title}>{event.title}</span>
        </div>
      ))}
      {events.length > shown.length && <div class="ev rest"><span>{t('list.moreEvents', { n: events.length - shown.length })}</span></div>}
    </div>
  );
}
