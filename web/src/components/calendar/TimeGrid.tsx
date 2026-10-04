// Day and week views: one scrolling 24-hour timeline, a column per day.
import type { CSSProperties, JSX, TargetedMouseEvent } from 'preact';
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import type { CalEvent, FocusSession } from '../../api/types';
import { t } from '../../i18n';
import { allDayOn, eventSpan } from '../../lib/calendar';
import { addDays, atMinutes, hhmm, minutesOfDay, mondayIndex, snap, spanOnDay, toUtc, ymd, type HourSpan, wall } from '../../lib/dates';
import { weekdayShort } from '../../lib/format';
import { layoutLanes, type Lane } from '../../lib/lanes';
import { calPop, justClosed, moveEvent, toggleBlockDone } from '../../state/calendar';
import { eventKey } from '../../state/events';
import { serverNow, serverTime, sessionTitle } from '../../state/focus';
import { navigate } from '../../state/route';
import { colorOf, now, suggestions, today } from '../../state/store';
import { timeRange } from '../../state/suggestions';
import { CalPopover } from './pops';

const DAY_HOUR = 58;
const WEEK_HOUR = 48;
/** A fresh view opens a little before the working day. */
const OPEN_AT_HOUR = 7.4;
const SNAP_MINUTES = 15;
const DAY_MINUTES = 24 * 60;
/** How long a finger must rest on a block before it can be dragged; a quicker move scrolls. */
const TOUCH_HOLD_MS = 350;

interface Drag {
  key: string;
  start: Date;
  end: Date;
}

/** Whether a tentative entry stands for an item rather than an event. */
function isTask(event: CalEvent): boolean {
  if (event.item_id) return true;
  const kind = suggestions.value.find(s => s.id === event.suggestion_id)?.kind;
  return kind === 'schedule_item' || kind === 'create_item';
}

/** Confirmed, editable entries within one day can be dragged. */
function canDrag(event: CalEvent): boolean {
  if (event.status !== 'confirmed' || event.readonly || event.all_day || !event.start || !event.end) return false;
  const start = wall(event.start);
  const end = wall(event.end);
  return end > start && (ymd(start) === ymd(end) || (minutesOfDay(end) === 0 && ymd(end) === addDays(ymd(start), 1)));
}

interface BlockProps {
  event: CalEvent;
  span: HourSpan;
  lane: Lane;
  hour: number;
  week: boolean;
  day: string;
  onGrab: (pointer: PointerEvent, event: CalEvent, mode: 'move' | 'resize') => void;
  dragged: () => boolean;
}

function Block({ event, span, lane, hour, week, day, onGrab, dragged }: BlockProps): JSX.Element {
  const len = span.e - span.s;
  const pending = event.status !== 'confirmed';
  const task = isTask(event);
  const movable = canDrag(event);
  const cls = ['blk', task ? 'task' : 'event', len <= 0.5 ? 'short' : len >= 1 ? 'tall' : ''];
  if (pending) cls.push(event.status === 'tentative' ? 'tent' : 'leaving');
  if (event.item_done) cls.push('done');
  if (week && lane.of >= 3) cls.push('narrow');
  if (movable) cls.push('grab');
  const open = () => {
    if (dragged()) return;
    calPop.value = { kind: 'event', event, day };
  };
  return (
    <div
      class={cls.join(' ')} title={event.title} role="button" tabIndex={0}
      style={{
        '--c': colorOf(event.project_id),
        top: `${span.s * hour + 1}px`,
        // Very short events keep a readable minimum height.
        height: `${Math.max(len * hour - 2, 17)}px`,
        left: `calc(${(lane.lane / lane.of) * 100}% + 2px)`,
        width: `calc(${100 / lane.of}% - 4px)`,
      }}
      onClick={open}
      onKeyDown={e => { if (e.key === 'Enter' && e.target === e.currentTarget) open(); }}
      onPointerDown={e => { if (movable && !(e.target as Element).closest('.chk')) onGrab(e, event, 'move'); }}
    >
      {event.item_id && !pending && (
        <button
          class={`chk ${event.item_done ? 'on' : ''}`} role="checkbox" aria-checked={!!event.item_done}
          aria-label={t('item.complete', { title: event.title })}
          onClick={e => { e.stopPropagation(); void toggleBlockDone(event); }}
        />
      )}
      <span class="bx">
        <span class="bt">{event.title}</span>
        <span class="bm">{timeRange(event.start!, event.end!)}{pending ? t('cal.pendingMark') : ''}</span>
      </span>
      {movable && <span class="rz" onPointerDown={e => { e.stopPropagation(); onGrab(e, event, 'resize'); }} />}
    </div>
  );
}

interface ColumnProps {
  day: string;
  index: number;
  events: readonly CalEvent[];
  sessions: readonly FocusSession[];
  hour: number;
  week: boolean;
  onGrab: BlockProps['onGrab'];
  dragged: () => boolean;
}

/** What was actually done: a thin line in the project's colour beside the plan. A running session reaches to now. */
function Strips({ day, sessions, hour }: { day: string; sessions: readonly FocusSession[]; hour: number }): JSX.Element {
  // Only a session still running needs the moving clock.
  const nowMs = sessions.some(s => Date.parse(s.end) > serverTime()) ? serverNow() : Infinity;
  return (
    <>
      {sessions.map(s => {
        const end = Math.min(Date.parse(s.end), Math.max(nowMs, Date.parse(s.start)));
        const at = spanOnDay(wall(s.start), wall(new Date(end).toISOString()), day);
        return at && (
          <i
            key={s.id} class="fs" title={t('focus.session', { range: timeRange(s.start, new Date(end).toISOString()), title: sessionTitle(s) })}
            style={{ '--c': colorOf(s.project_id), top: `${at.s * hour}px`, height: `${Math.max((at.e - at.s) * hour, 3)}px` }}
          />
        );
      })}
    </>
  );
}

function Column({ day, index, events, sessions, hour, week, onGrab, dragged }: ColumnProps): JSX.Element {
  // In time order, so that the keyboard walks through the day as the eye does.
  const timed = events.flatMap(event => {
    const span = eventSpan(event, day);
    return span ? [{ event, ...span }] : [];
  }).sort((a, b) => a.s - b.s || b.e - a.e);
  const lanes = layoutLanes(timed);
  const pop = calPop.value;
  const right = week && index >= 4;
  const side = right ? 'right:3px;transform-origin:calc(100% - 24px) 0' : 'left:2px';
  let popTop = 0;
  if (pop?.day === day) {
    if (pop.kind === 'new') {
      const [h, m] = pop.draft.end.split(':').map(Number);
      popTop = (h! * 60 + m!) / 60 || 24;
    } else popTop = timed.find(x => eventKey(x.event) === eventKey(pop.event))?.e ?? 0;
  }

  const create = (e: TargetedMouseEvent<HTMLDivElement>) => {
    if (e.target !== e.currentTarget || justClosed()) return;
    const y = e.clientY - e.currentTarget.getBoundingClientRect().top;
    const minutes = Math.min(Math.max(snap((y / hour) * 60, SNAP_MINUTES), 0), DAY_MINUTES - 60);
    calPop.value = {
      kind: 'new',
      day,
      draft: { title: '', allDay: false, date: day, endDate: day, start: hhmm(atMinutes(day, minutes)), end: hhmm(atMinutes(day, minutes + 60)), projectId: null, notes: '' },
    };
  };

  return (
    <div class={`col ${week && index >= 5 ? 'we' : ''}`} data-day={day} style={{ height: `${24 * hour}px` }} onClick={create}>
      <Strips day={day} sessions={sessions} hour={hour} />
      {timed.map(x => (
        <Block key={eventKey(x.event)} event={x.event} span={x} lane={lanes.get(x)!} hour={hour} week={week} day={day} onGrab={onGrab} dragged={dragged} />
      ))}
      {day === today.value && <div class="now" style={{ top: `${(minutesOfDay(now.value) / 60) * hour}px` }} />}
      <CalPopover day={day} place={`top:${popTop * hour + 6}px;${side}`} />
    </div>
  );
}

function Hours({ hour, showNow }: { hour: number; showNow: boolean }): JSX.Element {
  return (
    <div class="hours">
      {Array.from({ length: 24 }, (_, h) => <div>{`${String(h).padStart(2, '0')}:00`}</div>)}
      {showNow && <span class="nowcap" style={{ top: `${(minutesOfDay(now.value) / 60) * hour}px` }}>{hhmm(now.value)}</span>}
    </div>
  );
}

/** Where the popover of an all-day entry hangs: the all-day row, not a day column. */
const ALL_DAY = 'allday';

function AllDay({ event, style }: { event: CalEvent; style?: CSSProperties }): JSX.Element {
  return (
    <button class="ad" title={event.title} style={{ '--c': colorOf(event.project_id), ...style }} onClick={() => { calPop.value = { kind: 'event', event, day: ALL_DAY }; }}>
      {event.title}
    </button>
  );
}

export function TimeGrid({ days, events, sessions }: { days: string[]; events: readonly CalEvent[]; sessions: readonly FocusSession[] }): JSX.Element {
  const week = days.length > 1;
  const hour = week ? WEEK_HOUR : DAY_HOUR;
  const scroller = useRef<HTMLDivElement>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const dragging = useRef(false);
  const wasDragged = useRef(false);

  // Open a little before the working day. The grid can mount while its mode is still hidden
  // (a hidden element cannot scroll), so try again once the page has settled.
  useLayoutEffect(() => {
    const open = () => { if (scroller.current) scroller.current.scrollTop = OPEN_AT_HOUR * hour; };
    open();
    const retry = setTimeout(() => { if (scroller.current?.scrollTop === 0) open(); }, 0);
    return () => clearTimeout(retry);
  }, []);

  // While a block is being dragged by touch, the page must not scroll under the finger.
  useEffect(() => {
    const el = scroller.current!;
    const hold = (e: TouchEvent) => { if (dragging.current) e.preventDefault(); };
    el.addEventListener('touchmove', hold, { passive: false });
    return () => el.removeEventListener('touchmove', hold);
  }, []);

  const grab = (down: PointerEvent, event: CalEvent, mode: 'move' | 'resize') => {
    if (down.button !== 0) return;
    const el = down.currentTarget as HTMLElement;
    const box = scroller.current!;
    const start = wall(event.start!);
    const end = wall(event.end!);
    const startDay = ymd(start);
    const startMin = minutesOfDay(start);
    const length = Math.round((end.getTime() - start.getTime()) / 60_000);
    const scroll0 = box.scrollTop;
    const touch = down.pointerType === 'touch';
    let active = false;
    let latest: Drag | null = null;

    const activate = () => {
      active = true;
      dragging.current = true;
      wasDragged.current = true;
      // Capture keeps the moves coming when the pointer leaves the block; without it the
      // window listeners below still follow the drag.
      try { el.setPointerCapture(down.pointerId); } catch { /* the pointer is already gone */ }
    };
    const timer = touch ? setTimeout(activate, TOUCH_HOLD_MS) : undefined;

    const move = (e: PointerEvent) => {
      if (e.pointerId !== down.pointerId) return;
      const dx = e.clientX - down.clientX;
      const dy = e.clientY - down.clientY + (box.scrollTop - scroll0);
      if (!active) {
        const far = Math.hypot(dx, dy);
        if (touch) {
          // The finger moved before the hold elapsed: this is a scroll.
          if (far > 8) finish(false);
          return;
        }
        if (far < 4) return;
        activate();
      }
      const delta = snap((dy / hour) * 60, SNAP_MINUTES);
      if (mode === 'resize') {
        const newLength = Math.min(Math.max(length + delta, SNAP_MINUTES), DAY_MINUTES - startMin);
        latest = { key: eventKey(event), start, end: atMinutes(startDay, startMin + newLength) };
      } else {
        const cols = [...box.querySelectorAll<HTMLElement>('.col')];
        const over = cols.find(c => { const r = c.getBoundingClientRect(); return e.clientX >= r.left && e.clientX < r.right; });
        const day = over?.dataset.day ?? (latest ? ymd(latest.start) : startDay);
        const from = Math.min(Math.max(startMin + delta, 0), DAY_MINUTES - SNAP_MINUTES);
        latest = { key: eventKey(event), start: atMinutes(day, from), end: atMinutes(day, from + length) };
      }
      setDrag(latest);
    };
    const finish = (commit: boolean) => {
      clearTimeout(timer);
      removeEventListener('pointermove', move);
      removeEventListener('pointerup', up);
      removeEventListener('pointercancel', cancel);
      dragging.current = false;
      // The click that follows a drag must not open the popover.
      setTimeout(() => { wasDragged.current = false; }, 0);
      setDrag(null);
      if (commit && active && latest && (latest.start.getTime() !== start.getTime() || latest.end.getTime() !== end.getTime())) {
        void moveEvent(event, toUtc(latest.start), toUtc(latest.end));
      }
    };
    const up = (e: PointerEvent) => { if (e.pointerId === down.pointerId) finish(true); };
    const cancel = (e: PointerEvent) => { if (e.pointerId === down.pointerId) finish(false); };
    addEventListener('pointermove', move);
    addEventListener('pointerup', up);
    addEventListener('pointercancel', cancel);
  };

  // The block being dragged is drawn where it would land.
  const shown = drag ? events.map(e => (eventKey(e) === drag.key ? { ...e, start: toUtc(drag.start), end: toUtc(drag.end) } : e)) : events;
  const dragged = () => wasDragged.current;
  const hasToday = days.includes(today.value);
  const columns = days.map((day, index) => (
    <Column key={day} day={day} index={index} events={shown} sessions={sessions} hour={hour} week={week} onGrab={grab} dragged={dragged} />
  ));

  if (!week) {
    const day = days[0]!;
    const all = events.filter(e => allDayOn(e, day));
    return (
      <>
        {all.length > 0 && (
          <div class="allday">
            <span>{t('cal.allDay')}</span>
            <div>{all.map(e => <AllDay key={eventKey(e)} event={e} />)}</div>
            <CalPopover day={ALL_DAY} place="top:100%;left:52px" />
          </div>
        )}
        <div class="cal-scroll" ref={scroller}>
          <div class="tl"><Hours hour={hour} showNow={hasToday} />{columns}</div>
        </div>
      </>
    );
  }

  const first = days[0]!;
  const last = days[6]!;
  // All-day and multi-day events sit in a row under the dates and span the days they cover.
  const all = events.filter(e => e.all_day && e.start_date && e.start_date <= last && (e.end_date ?? e.start_date) >= first);
  const firstColumn = (e: CalEvent) => (e.start_date! < first ? 0 : mondayIndex(e.start_date!));
  const pop = calPop.value;
  const popColumn = pop?.kind === 'event' && pop.day === ALL_DAY ? firstColumn(pop.event) : 0;
  return (
    <div class="cal-scroll" ref={scroller}>
      <div class="wk">
        <div class="wk-top">
          <div class="wk-gap" />
          {days.map(day => (
            <button class={`wk-head ${day === today.value ? 'today' : ''}`} onClick={() => navigate({ view: 'day', date: day })}>
              {weekdayShort(day)}<b>{Number(day.slice(8))}</b>
            </button>
          ))}
          {all.length > 0 && (
            <>
              <div class="wk-adl">{t('cal.allDay')}</div>
              <div class="wk-ad">
                {all.map(e => {
                  const end = e.end_date ?? e.start_date!;
                  const to = end > last ? 6 : mondayIndex(end);
                  return <AllDay key={eventKey(e)} event={e} style={{ gridColumn: `${firstColumn(e) + 1} / ${to + 2}` }} />;
                })}
                <CalPopover day={ALL_DAY} place={popColumn >= 4 ? 'top:100%;right:3px;transform-origin:calc(100% - 24px) 0' : `top:100%;left:calc(${(popColumn / 7) * 100}% + 2px)`} />
              </div>
            </>
          )}
        </div>
        <Hours hour={hour} showNow={hasToday} />
        {columns}
      </div>
    </div>
  );
}
