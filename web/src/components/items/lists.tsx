// The built-in lists: inbox, today, upcoming, the four quadrants, everything, and the log.
import type { JSX } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { Item, ItemPage, Quadrant, QuadrantKey } from '../../api/types';
import { t, type MessageKey } from '../../i18n';
import { addDays, addMonths, daysBetween, monthEnd, monthStart } from '../../lib/dates';
import { groupByMonth } from '../../lib/done';
import { dayLabel, weekdayShort } from '../../lib/format';
import { todaySummary } from '../../lib/summary';
import { current, draft } from '../../state/items';
import { track, useResource } from '../../state/resource';
import { navigate } from '../../state/route';
import { colorOf, projectOf, reportError, today, user, version } from '../../state/store';
import { Icon, Pie } from '../Icons';
import { DraftRow } from './ItemRow';
import { Blank, EventsStrip, ListHead, LoadFailed, MoreButton, Rows, Skeleton } from './parts';
import { usePaged } from './usePaged';

export function Inbox(): JSX.Element {
  const page = usePaged({ project_id: 'none' }, { first: 20, step: 60 });
  const head = <ListHead icon={<Icon name="tray" />} name={t('nav.inbox')} color="var(--blue)" />;
  if (page.failed) return <>{head}<LoadFailed retry={page.reload} /></>;
  if (!page.items) return <>{head}<Skeleton /></>;
  if (!page.items.length && !draft.value) return <>{head}<Blank title={t('inbox.empty')} tip={t('inbox.emptyTip')} /></>;
  return (
    <>
      {head}
      <p class="summary">{t('inbox.summary')}</p>
      <DraftRow />
      <Rows items={page.items} />
      <MoreButton rest={page.rest} step={60} loading={page.loading} onMore={page.more} />
    </>
  );
}

export function Today(): JSX.Element {
  // Keyed by the day, so the list is fetched again when the date changes at midnight.
  const res = useResource(`today:${today.value}`, api.getToday);
  const head = <ListHead icon={<Icon name="star" />} name={t('nav.today')} color="#f5b400" />;
  const data = res.data;
  if (res.failed) return <>{head}<LoadFailed retry={res.reload} /></>;
  if (!data) return <>{head}<Skeleton /></>;

  const items = data.items.map(current);
  const open = (item: Item) => item.status === 'open';
  // Important items lead the day.
  const day = items.filter(i => !i.evening).sort((a, b) => Number(b.important) - Number(a.important));
  const night = items.filter(i => i.evening);
  if (!items.length && !data.overdue.length && !data.events.length && !draft.value) {
    return <>{head}<Blank title={t('today.empty')} tip={t('today.emptyTip')} /></>;
  }
  const gaps = day.filter(i => open(i) && !i.block && !i.suggestion);
  const summary = todaySummary({
    freeMinutes: data.free_minutes,
    unplannedMinutes: data.unplanned_minutes,
    unplanned: gaps.length,
    important: gaps.filter(i => i.important).length,
    workEnd: user.value?.work_end ?? '18:00',
  });
  return (
    <>
      {head}
      <p class="summary">{summary}</p>
      <EventsStrip events={data.events} />
      <DraftRow />
      {data.overdue.length > 0 && (
        <>
          <div class="sec" style={{ '--c': 'var(--red)' }}><Icon name="flag" /><span>{t('today.overdue')}</span><em>{data.overdue.length}</em></div>
          <Rows items={data.overdue} opts={{ sub: true }} />
          <div class="sec"><span>{t('nav.today')}</span><em>{day.filter(open).length}</em></div>
        </>
      )}
      <Rows items={day} opts={{ sub: true }} />
      {night.length > 0 && (
        <>
          <div class="sec" style={{ '--c': 'var(--indigo)' }}><Icon name="moon" /><span>{t('when.tonight')}</span></div>
          <Rows items={night} opts={{ sub: true }} />
        </>
      )}
    </>
  );
}

/** Days this close to today are listed even when they only have calendar events. */
const NEAR_DAYS = 4;
const MAX_MONTHS_AHEAD = 24;

function UpcomingRange({ from, to, first }: { from: string; to: string; first: boolean }): JSX.Element {
  const res = useResource(`upcoming:${from}:${to}`, () => api.getUpcoming(from, to));
  const day = today.value;
  if (res.failed) return <LoadFailed retry={res.reload} />;
  if (!res.data) return <Skeleton />;
  const days = res.data.filter(d => d.items.length || (d.events.length && daysBetween(day, d.date) <= NEAR_DAYS));
  if (!days.length) return first && !draft.value ? <Blank title={t('upcoming.empty')} tip={t('upcoming.emptyTip')} /> : <></>;
  return (
    <>
      {days.map(d => {
        const [y, m, date] = d.date.split('-').map(Number);
        const sameMonth = d.date.slice(0, 7) === day.slice(0, 7);
        const month = y === Number(day.slice(0, 4)) ? t('date.m', { m: m! }) : t('date.ym', { y: y!, m: m! });
        return (
          <div key={d.date}>
            <div class="dh"><b>{date}</b><span>{sameMonth ? dayLabel(d.date, day) : `${month} · ${weekdayShort(d.date)}`}</span></div>
            <EventsStrip events={d.events} max={4} />
            <Rows items={d.items} opts={{ sub: true, timeOnly: true }} />
          </div>
        );
      })}
    </>
  );
}

export function Upcoming(): JSX.Element {
  // Only the current month is fetched up front; later months come on request.
  const [extra, setExtra] = useState(0);
  const from = addDays(today.value, 1);
  const next = addMonths(monthStart(from), extra + 1);
  const months = Array.from({ length: extra }, (_, k) => addMonths(monthStart(from), k + 1));
  return (
    <>
      <ListHead icon={<Icon name="cal" />} name={t('nav.upcoming')} color="var(--red)" />
      <DraftRow />
      <UpcomingRange key={from} from={from} to={monthEnd(from)} first />
      {months.map(m => <UpcomingRange key={m} from={m} to={monthEnd(m)} first={false} />)}
      {extra < MAX_MONTHS_AHEAD && (
        <button class="morebtn" style="padding-left:2px;margin-top:10px" onClick={() => setExtra(extra + 1)}>
          {t('upcoming.load', { month: next.slice(0, 4) === today.value.slice(0, 4) ? t('date.m', { m: Number(next.slice(5, 7)) }) : t('date.ym', { y: Number(next.slice(0, 4)), m: Number(next.slice(5, 7)) }) })}
        </button>
      )}
    </>
  );
}

const QUADS: ReadonlyArray<{ key: QuadrantKey; name: MessageKey; label: MessageKey; color: string }> = [
  { key: 'do', name: 'matrix.do', label: 'matrix.doLabel', color: 'var(--red)' },
  { key: 'plan', name: 'matrix.plan', label: 'matrix.planLabel', color: 'var(--blue)' },
  { key: 'quick', name: 'matrix.quick', label: 'matrix.quickLabel', color: 'var(--orange)' },
  { key: 'later', name: 'matrix.later', label: 'matrix.laterLabel', color: 'var(--gray)' },
];

function Quad({ spec, data }: { spec: (typeof QUADS)[number]; data: Quadrant }): JSX.Element {
  const page = usePaged({ quadrant: spec.key }, { first: 6, step: 18, initial: data });
  const items = page.items ?? [];
  return (
    <div class="quad">
      <div class="quad-h" style={{ '--c': spec.color }}>
        <b>{t(spec.name)}</b>
        <em>{t(spec.label)}</em>
        <span>{t('matrix.count', { n: data.total })}{spec.key === 'plan' && data.unplanned > 0 ? t('matrix.unplanned', { n: data.unplanned }) : ''}</span>
      </div>
      {items.length ? <Rows items={items} opts={{ sub: true }} /> : <div class="none">{t('matrix.none')}</div>}
      <MoreButton rest={page.rest} step={18} loading={page.loading} onMore={page.more} />
    </div>
  );
}

export function Matrix(): JSX.Element {
  const res = useResource('matrix', api.getMatrix);
  const head = <ListHead icon={<Icon name="grid" />} name={t('nav.matrix')} color="var(--orange)" />;
  const data = res.data;
  if (res.failed) return <>{head}<LoadFailed retry={res.reload} /></>;
  if (!data) return <>{head}<Skeleton /></>;
  if (!QUADS.some(q => data[q.key].total) && !draft.value) return <>{head}<Blank title={t('all.empty')} tip={t('matrix.emptyTip')} /></>;
  return (
    <>
      {head}
      <p class="summary">{t('matrix.summary')}</p>
      <DraftRow />
      <div class="matrix">{QUADS.map(q => <Quad key={q.key} spec={q} data={data[q.key]} />)}</div>
    </>
  );
}

function OverviewSection({ projectId, total, items }: { projectId: string; total: number; items: Item[] }): JSX.Element | null {
  const page = usePaged({ project_id: projectId }, { first: 5, step: 15, initial: { items, total } });
  const project = projectOf(projectId);
  if (!project) return null;
  return (
    <>
      <button class="sec" onClick={() => navigate({ mode: 'items', list: `p:${projectId}` })}>
        <Pie done={project.done_count} open={project.open_count} color={colorOf(projectId)} />
        <span title={project.name}>{project.name}</span>
        <em>{total}</em>
      </button>
      <Rows items={page.items ?? items} opts={{ star: true }} />
      <MoreButton rest={page.rest} step={15} loading={page.loading} onMore={page.more} />
    </>
  );
}

export function All(): JSX.Element {
  const res = useResource('overview', api.getOverview);
  const head = <ListHead icon={<Icon name="layers" />} name={t('nav.all')} color="var(--teal)" />;
  if (res.failed) return <>{head}<LoadFailed retry={res.reload} /></>;
  if (!res.data) return <>{head}<Skeleton /></>;
  const sections = res.data.filter(p => p.total > 0);
  if (!sections.length && !draft.value) return <>{head}<Blank title={t('all.empty')} tip={t('all.emptyTip')} /></>;
  return (
    <>
      {head}
      <DraftRow />
      {sections.map(p => <OverviewSection key={p.project_id} projectId={p.project_id} total={p.total} items={p.items} />)}
    </>
  );
}

const DONE_PAGE = 50;
/** How close to the viewport's bottom edge the end of the log must be to fetch more. */
const NEAR_END = 120;

export function Done(): JSX.Element {
  const [page, setPage] = useState<ItemPage | undefined>(undefined);
  const [failed, setFailed] = useState(false);
  const [nonce, setNonce] = useState(0);
  const sentinel = useRef<HTMLDivElement>(null);
  // Bumped by every full reload, so a page that arrives for an older list is dropped.
  const generation = useRef(0);
  const busy = useRef(false);
  const shown = useRef(DONE_PAGE);
  const v = version.value;

  useEffect(() => {
    const mine = ++generation.current;
    track(api.listItemsUpTo({ status: 'done' }, shown.current)).then(
      res => {
        if (mine !== generation.current) return;
        busy.current = false;
        setFailed(false);
        setPage(res);
      },
      err => {
        if (mine !== generation.current) return;
        busy.current = false;
        reportError(err);
        setFailed(true);
      },
    );
  }, [v, nonce]);

  // Newest first; reaching the bottom of the page asks for the next batch.
  useEffect(() => {
    const fetchIfNear = () => {
      const el = sentinel.current;
      if (!el || busy.current || !page?.next_cursor || el.getBoundingClientRect().top > innerHeight + NEAR_END) return;
      busy.current = true;
      const mine = generation.current;
      api.listItems({ status: 'done', limit: DONE_PAGE, cursor: page.next_cursor }).then(
        more => {
          if (mine !== generation.current) return;
          busy.current = false;
          shown.current = page.items.length + more.items.length;
          setPage({ items: [...page.items, ...more.items], total: more.total, next_cursor: more.next_cursor });
        },
        err => {
          if (mine !== generation.current) return;
          reportError(err);
          // Stay idle until the user scrolls again rather than retrying in a loop.
          setTimeout(() => { busy.current = false; }, 3000);
        },
      );
    };
    fetchIfNear();
    addEventListener('scroll', fetchIfNear, { passive: true });
    return () => removeEventListener('scroll', fetchIfNear);
  }, [page]);

  const head = <ListHead icon={<Icon name="book" />} name={t('nav.done')} color="var(--green)" />;
  if (!page) return <>{head}{failed ? <LoadFailed retry={() => setNonce(n => n + 1)} /> : <Skeleton />}</>;
  if (!page.items.length) return <>{head}<DraftRow /><Blank title={t('done.empty')} tip={t('done.emptyTip')} /></>;
  const groups = groupByMonth(page.items);
  const thisMonth = today.value.slice(0, 7);
  return (
    <>
      {head}
      <p class="summary">{t('done.summary', { n: page.total })}</p>
      <DraftRow />
      {groups.map((group, k) => {
        const [y, m] = group.month.split('-').map(Number);
        const label = group.month === thisMonth ? t('done.thisMonth') : y === Number(thisMonth.slice(0, 4)) ? t('date.m', { m: m! }) : t('date.ym', { y: y!, m: m! });
        // The last month may continue on the next page, so its count is not known yet.
        const complete = k < groups.length - 1 || !page.next_cursor;
        return (
          <div key={group.month}>
            <div class="sec"><span>{label}</span>{complete && <em>{group.items.length}</em>}</div>
            <Rows items={group.items} opts={{ sub: true, doneDate: true }} />
          </div>
        );
      })}
      {page.next_cursor ? <div ref={sentinel}><Skeleton /></div> : <p class="end">{t('done.end')}</p>}
    </>
  );
}
