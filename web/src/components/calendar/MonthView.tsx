// Month view: a cell per day listing what is on it, capped at four rows.
import type { JSX, TargetedMouseEvent } from 'preact';
import type { CalEvent } from '../../api/types';
import { t } from '../../i18n';
import { allDayOn, eventSpan } from '../../lib/calendar';
import { hhmm, monthGrid } from '../../lib/dates';
import { pickMonthEntries } from '../../lib/month';
import { calPop, justClosed } from '../../state/calendar';
import { eventKey } from '../../state/events';
import { navigate } from '../../state/route';
import { colorOf, suggestions, today } from '../../state/store';
import { CalPopover } from './pops';

interface Entry {
  event: CalEvent;
  allDay: boolean;
  pending: boolean;
  s: number;
}

function entriesOn(events: readonly CalEvent[], day: string): Entry[] {
  return events.flatMap((event): Entry[] => {
    const pending = event.status !== 'confirmed';
    if (allDayOn(event, day)) return [{ event, allDay: true, pending, s: 0 }];
    const span = eventSpan(event, day);
    return span ? [{ event, allDay: false, pending, s: span.s }] : [];
  });
}

function MonthEntry({ entry, day }: { entry: Entry; day: string }): JSX.Element {
  const { event } = entry;
  const style = { '--c': colorOf(event.project_id) };
  const open = () => { calPop.value = { kind: 'event', event, day }; };
  if (entry.allDay) return <button class="me ad" title={event.title} style={style} onClick={open}>{event.title}</button>;
  const kind = suggestions.value.find(s => s.id === event.suggestion_id)?.kind;
  const task = event.item_id !== null || kind === 'schedule_item' || kind === 'create_item';
  const cls = ['me', task ? 'task' : 'event', entry.pending ? (event.status === 'tentative' ? 'tent' : 'leaving') : '', event.item_done ? 'done' : ''];
  return (
    <button class={cls.join(' ')} title={event.title} style={style} onClick={open}>
      <i />
      <span class="mt">{event.title}</span>
      <span class="mx">{entry.pending ? t('cal.pending') : hhmm(new Date(event.start!))}</span>
    </button>
  );
}

export function MonthView({ date, events }: { date: string; events: readonly CalEvent[] }): JSX.Element {
  const cells = monthGrid(date);
  const names = t('day.short').split(',');

  const create = (e: TargetedMouseEvent<HTMLDivElement>, day: string) => {
    if (e.target !== e.currentTarget || justClosed()) return;
    calPop.value = {
      kind: 'new',
      day,
      draft: { title: '', allDay: true, date: day, endDate: day, start: '09:00', end: '10:00', projectId: null, notes: '' },
    };
  };

  return (
    <>
      <div class="mo-head">{names.map(n => <div>{n}</div>)}</div>
      <div class="mo">
        {cells.map((cell, c) => {
          const [, m, d] = cell.date.split('-').map(Number);
          const { visible, more } = pickMonthEntries(cell.inMonth ? entriesOn(events, cell.date) : []);
          const side = c % 7 >= 4 ? 'right:3px;transform-origin:calc(100% - 24px) 0' : 'left:2px';
          const cls = ['mcell', c % 7 >= 5 ? 'we' : '', cell.inMonth ? '' : 'out', cell.date === today.value ? 'today' : ''];
          return (
            <div key={cell.date} class={cls.join(' ')} onClick={e => { if (cell.inMonth) create(e, cell.date); }}>
              <div class="mday">
                <button aria-label={t('date.md', { m: m!, d: d! })} onClick={() => navigate({ view: 'day', date: cell.date })}>
                  <b>{cell.inMonth && d === 1 ? t('date.md', { m: m!, d: 1 }) : d}</b>
                </button>
              </div>
              {visible.map(entry => <MonthEntry key={eventKey(entry.event)} entry={entry} day={cell.date} />)}
              {more > 0 && <button class="more" onClick={() => navigate({ view: 'day', date: cell.date })}>{t('cal.more', { n: more })}</button>}
              <CalPopover day={cell.date} place={`top:calc(100% - 8px);${side}`} />
            </div>
          );
        })}
      </div>
    </>
  );
}
