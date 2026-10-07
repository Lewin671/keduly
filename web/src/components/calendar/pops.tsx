// The popovers of the calendar: a suggestion to decide, the event editor, a time block's actions.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { CalEvent, EventWrite } from '../../api/types';
import { keepComposing } from '../../lib/keys';
import { t } from '../../i18n';
import { addDays, atLocal, hhmm, toUtc, ymd, wall } from '../../lib/dates';
import { dayLabel } from '../../lib/format';
import { calPop, closeCalPop, type EventDraft } from '../../state/calendar';
import { openTimer, startFocus, workingOn } from '../../state/focus';
import { openItem } from '../../state/items';
import { navigate } from '../../state/route';
import { activeProjects, colorOf, projectOf, suggestions, today, write } from '../../state/store';
import { decide, describe, timeRange } from '../../state/suggestions';
import { Popover } from '../Popover';

export function draftFor(event: CalEvent): EventDraft {
  const start = event.start ? wall(event.start) : null;
  const end = event.end ? wall(event.end) : null;
  const date = event.all_day ? event.start_date! : ymd(start!);
  return {
    title: event.title,
    allDay: event.all_day,
    date,
    endDate: event.all_day ? (event.end_date ?? date) : date,
    start: start ? hhmm(start) : '09:00',
    end: end ? hhmm(end) : '10:00',
    projectId: event.project_id,
    notes: event.notes,
  };
}

/** The instants of a draft's start and end. An end at or before the start means the next day. */
function instants(d: Pick<EventDraft, 'date' | 'start' | 'end'>): [string, string] {
  const start = atLocal(d.date, d.start);
  let end = atLocal(d.date, d.end);
  if (end <= start) end = atLocal(addDays(d.date, 1), d.end);
  return [toUtc(start), toUtc(end)];
}

function timesOf(d: EventDraft): EventWrite {
  if (d.allDay) return { all_day: true, start_date: d.date, end_date: d.endDate < d.date ? d.date : d.endDate };
  const [start, end] = instants(d);
  return { all_day: false, start, end };
}

function ProjectSelect({ value, onChange }: { value: string | null; onChange: (id: string | null) => void }): JSX.Element {
  return (
    <label class="lbl">{t('field.project')}
      <select class="fld" value={value ?? ''} onChange={event => onChange(event.currentTarget.value || null)}>
        <option value="">{t('project.none')}</option>
        {activeProjects.value.map(p => <option value={p.id}>{p.name}</option>)}
      </select>
    </label>
  );
}

function EventEditor({ event, initial }: { event?: CalEvent; initial: EventDraft }): JSX.Element {
  const [d, setD] = useState(initial);
  const [confirming, setConfirming] = useState(false);
  const set = (change: Partial<EventDraft>) => setD({ ...d, ...change });

  const save = () => {
    if (!d.date || (!d.allDay && (!d.start || !d.end))) return;
    const body = { title: d.title.trim() || t('event.untitled'), project_id: d.projectId, notes: d.notes, ...timesOf(d) };
    closeCalPop();
    void write(event ? api.updateEvent(event.id, body) : api.createEvent(body));
  };
  const remove = () => {
    closeCalPop();
    void write(api.deleteEvent(event!.id));
  };

  return (
    <form class="form" onKeyDown={keepComposing} onSubmit={e => { e.preventDefault(); save(); }}>
      <input class="fld title" value={d.title} placeholder={t('event.title')} aria-label={t('event.title')} maxLength={500} autoFocus onInput={e => set({ title: e.currentTarget.value })} />
      <label class="switch"><span>{t('cal.allDay')}</span><input type="checkbox" checked={d.allDay} onChange={e => set({ allDay: e.currentTarget.checked })} /></label>
      {d.allDay ? (
        <>
          <input class="fld" type="date" required value={d.date} aria-label={t('field.startDate')} onInput={e => set({ date: e.currentTarget.value })} />
          <input class="fld" type="date" required value={d.endDate} min={d.date} aria-label={t('field.endDate')} onInput={e => set({ endDate: e.currentTarget.value })} />
        </>
      ) : (
        <>
          <input class="fld" type="date" required value={d.date} aria-label={t('field.date')} onInput={e => set({ date: e.currentTarget.value })} />
          <div class="frow">
            <input class="fld grow" type="time" required step={300} value={d.start} aria-label={t('field.start')} onInput={e => set({ start: e.currentTarget.value })} />
            <span>–</span>
            <input class="fld grow" type="time" required step={300} value={d.end} aria-label={t('field.end')} onInput={e => set({ end: e.currentTarget.value })} />
          </div>
        </>
      )}
      <ProjectSelect value={d.projectId} onChange={projectId => set({ projectId })} />
      <textarea class="fld area" rows={2} value={d.notes} placeholder={t('item.notes')} aria-label={t('item.notes')} onInput={e => set({ notes: e.currentTarget.value })} />
      <div class="pa">
        {event && (confirming
          ? <button type="button" class="pbtn danger" onClick={remove}>{t('common.confirmDelete')}</button>
          : <button type="button" class="pbtn" onClick={() => setConfirming(true)}>{t('common.delete')}</button>)}
        <button type="submit" class="pbtn go">{t('common.save')}</button>
      </div>
    </form>
  );
}

/** When an entry takes place, in words. */
function whenText(event: CalEvent): string {
  const day = today.value;
  if (event.all_day) {
    const from = dayLabel(event.start_date!, day);
    return event.end_date && event.end_date !== event.start_date ? `${from} – ${dayLabel(event.end_date, day)} · ${t('cal.allDay')}` : `${from} · ${t('cal.allDay')}`;
  }
  return `${dayLabel(ymd(wall(event.start!)), day)} ${timeRange(event.start!, event.end!)}`;
}

function SuggestionPop({ event }: { event: CalEvent }): JSX.Element {
  const s = suggestions.value.find(x => x.id === event.suggestion_id);
  const project = projectOf(event.project_id);
  const decideIt = (accept: boolean) => {
    closeCalPop();
    void decide(event.suggestion_id!, event.title, accept);
  };
  return (
    <>
      <div class="pt"><i />{event.title}</div>
      <div class="pw">{s ? describe(s) : whenText(event)}</div>
      {s && (
        <div class="pr">
          <span>{t('suggest.by', { who: s.actor.name })}{project ? ` · ${project.name}` : ''}</span>
          {s.reason}
        </div>
      )}
      <div class="pa">
        <button class="pbtn" onClick={() => decideIt(false)}>{t('suggest.reject')}</button>
        <button class="pbtn go" onClick={() => decideIt(true)}>{t('suggest.accept')}</button>
      </div>
    </>
  );
}

/** An instance of a recurring event: shown, not edited. */
function DetailsPop({ event }: { event: CalEvent }): JSX.Element {
  const project = projectOf(event.project_id);
  return (
    <>
      <div class="pt"><i />{event.title}</div>
      <div class="pw">{whenText(event)}{project ? ` · ${project.name}` : ''}</div>
      {event.location && <div class="pw">{event.location}</div>}
      {event.notes && <div class="pr pre">{event.notes}</div>}
      <div class="pr"><span>{t('event.recurringNote')}</span></div>
    </>
  );
}

/** A time block: start a tomato on its item, change when the item is done, take it off the calendar, or go to the item. */
function BlockPop({ event }: { event: CalEvent }): JSX.Element {
  const [d, setD] = useState(() => draftFor(event));
  const itemId = event.item_id!;
  const save = () => {
    if (!d.date || !d.start || !d.end) return;
    closeCalPop();
    void write(api.scheduleItem(itemId, ...instants(d)));
  };
  const unschedule = () => {
    closeCalPop();
    void write(api.unscheduleItem(itemId));
  };
  const show = () => {
    closeCalPop();
    navigate({ mode: 'items', list: event.project_id ? `p:${event.project_id}` : 'inbox' });
    openItem(itemId);
  };
  const focusOn = () => {
    closeCalPop();
    if (workingOn(itemId)) openTimer();
    else void startFocus(itemId);
  };
  return (
    <form class="form" onSubmit={e => { e.preventDefault(); save(); }}>
      <div class="pt"><i />{event.title}</div>
      {!event.item_done && <button type="button" class="pbtn go" onClick={focusOn}>{t(workingOn(itemId) ? 'focus.back' : 'focus.start')}</button>}
      <input class="fld" type="date" required value={d.date} aria-label={t('field.date')} onInput={e => setD({ ...d, date: e.currentTarget.value })} />
      <div class="frow">
        <input class="fld grow" type="time" required step={300} value={d.start} aria-label={t('field.start')} onInput={e => setD({ ...d, start: e.currentTarget.value })} />
        <span>–</span>
        <input class="fld grow" type="time" required step={300} value={d.end} aria-label={t('field.end')} onInput={e => setD({ ...d, end: e.currentTarget.value })} />
      </div>
      <div class="pa">
        <button type="button" class="pbtn" onClick={unschedule}>{t('when.clear')}</button>
        <button type="submit" class="pbtn go">{t('common.save')}</button>
      </div>
      <div class="links"><button type="button" class="tb" onClick={show}>{t('event.openItem')}</button></div>
    </form>
  );
}

/** The open popover, when it belongs to `day` (a column or a month cell; `null` is the toolbar). */
export function CalPopover({ day, place }: { day: string | null; place: string }): JSX.Element | null {
  const pop = calPop.value;
  if (!pop || pop.kind === 'session' || pop.day !== day) return null;
  const event = pop.kind === 'event' ? pop.event : undefined;
  const key = event ? `${event.id}|${event.instance}|${event.status}` : 'new';
  return (
    <Popover key={key} within="self" onClose={closeCalPop} style={`--c:${colorOf(event?.project_id)};${place}`} label={event?.title ?? t('event.new')}>
      {pop.kind === 'new' ? <EventEditor initial={pop.draft} />
        : !event ? null
        : event.status !== 'confirmed' ? <SuggestionPop event={event} />
        : event.item_id ? <BlockPop event={event} />
        : event.readonly ? <DetailsPop event={event} />
        : <EventEditor event={event} initial={draftFor(event)} />}
    </Popover>
  );
}
