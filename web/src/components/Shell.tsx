// The signed-in app: sidebar, toolbar, and the calendar, the item lists or the focus timer.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import { t } from '../i18n';
import { stepDate, weekTitle, type CalView } from '../lib/calendar';
import { atMinutes, hhmm, minutesOfDay } from '../lib/dates';
import { cap, weekdayLong } from '../lib/format';
import { calPop } from '../state/calendar';
import { focus, focusColor, zen } from '../state/focus';
import { navigate, route } from '../state/route';
import * as api from '../api/client';
import { counts, keepZone, now, today, user, write, zoneQuestion } from '../state/store';
import { hideHud, hud, toast } from '../state/ui';
import { ActivityPanel } from './ActivityPanel';
import { Calendar } from './calendar/Calendar';
import { CalPopover } from './calendar/pops';
import { FocusMain } from './focus/FocusMain';
import { FocusCapsule } from './focus/parts';
import { Icon } from './Icons';
import { Dialog } from './Popover';
import { ItemsMain, NewItemButton } from './items/ItemsMain';
import { Settings } from './Settings';
import { setMode, Sidebar } from './Sidebar';

const VIEWS: ReadonlyArray<[CalView, 'view.day' | 'view.week' | 'view.month' | 'view.year']> = [
  ['day', 'view.day'], ['week', 'view.week'], ['month', 'view.month'], ['year', 'view.year'],
];

/** The calendar title: the bold part names the period, the light part adds to it. */
function Title({ view, date }: { view: CalView; date: string }): JSX.Element {
  const [y, m, d] = date.split('-').map(Number);
  if (view === 'day') {
    const sameYear = date.slice(0, 4) === today.value.slice(0, 4);
    return <><b>{sameYear ? t('date.md', { m: m!, d: d! }) : t('date.ymd', { y: y!, m: m!, d: d! })}</b>{weekdayLong(date)}</>;
  }
  if (view === 'week') {
    const w = weekTitle(date);
    return <><b>{t('date.ym', { y: w.year, m: w.month })}</b>{t('cal.week', { n: w.week })}</>;
  }
  return <b>{view === 'month' ? t('date.ym', { y: y!, m: m! }) : t('date.y', { y: y! })}</b>;
}

/** A new event from the toolbar starts at the next full hour. */
function newEvent(): void {
  const { view, date } = route.value;
  const day = view === 'day' ? date : today.value;
  const from = Math.min((Math.floor(minutesOfDay(now.value) / 60) + 1) * 60, 23 * 60);
  calPop.value = {
    kind: 'new',
    day: null,
    draft: { title: '', allDay: false, date: day, endDate: day, start: hhmm(atMinutes(day, from)), end: hhmm(atMinutes(day, from + 60)), projectId: null, notes: '' },
  };
}

function Hud(): JSX.Element | null {
  const h = hud.value;
  if (!h) return null;
  return (
    <div class="hud glass on" role="status">
      <span>{h.text}</span>
      {h.undo && <button onClick={() => { h.undo!(); hideHud(); }}>{h.label ?? t('activity.undo')}</button>}
    </div>
  );
}

export function Toast(): JSX.Element | null {
  return toast.value ? <div class="toast" role="alert">{toast.value}</div> : null;
}

/**
 * Asks before the account's time zone changes. Shown when the device is in another zone than the
 * account; closing it any way counts as "keep", and it does not come back on this device.
 */
function ZoneQuestion(): JSX.Element | null {
  const device = zoneQuestion.value;
  const me = user.value;
  if (!device || !me) return null;
  return (
    <Dialog onClose={keepZone} label={t('zone.title')} class="ask stack">
      <b>{t('zone.title')}</b>
      <p>{t('zone.text', { device, account: me.timezone })}</p>
      <div class="pa">
        <button class="pbtn go" onClick={() => { void write(api.updateMe({ timezone: device })); }}>{t('zone.switch', { device })}</button>
        <button class="pbtn" onClick={keepZone}>{t('zone.keep', { account: me.timezone })}</button>
      </div>
    </Dialog>
  );
}

export function Shell(): JSX.Element {
  const { mode, view, date } = route.value;
  const [panel, setPanel] = useState<'activity' | 'settings' | null>(null);
  const cal = mode === 'cal';
  const focusing = mode === 'focus';
  const pending = counts.value.pending;

  // The mockup's stylesheet switches between the modes on this attribute.
  // Set during render, not in an effect: children measure and scroll in their own layout effects,
  // which run before a parent's, and the stylesheet hides the inactive mode by this attribute.
  document.body.dataset.mode = cal ? 'cal' : focusing ? 'focus' : 'tasks';
  // While a timer is active its page takes the whole window, tinted with the timer's colour.
  document.body.toggleAttribute('data-zen', zen.value);
  document.body.style.setProperty('--zc', focus.value && focus.value.state !== 'idle' ? focusColor(focus.value) : 'transparent');

  return (
    <>
      <div class="app">
        <Sidebar />
        <main class="main">
          <div class="bar">
            <h1 id="title">{cal && <Title view={view} date={date} />}</h1>
            <div class="tools">
              {cal && (
                <>
                  <div class="seg only-cal" role="group" aria-label={t('view.label')}>
                    {VIEWS.map(([v, label]) => (
                      <button class={v === view ? 'on' : ''} aria-pressed={v === view} onClick={() => navigate({ view: v })}>{t(label)}</button>
                    ))}
                  </div>
                  <div class="cap nav-cap only-cal">
                    <button aria-label={t('cal.back')} onClick={() => navigate({ date: stepDate(view, date, -1) })}><Icon name="left" /></button>
                    <button class="red" onClick={() => navigate({ date: today.value })}>{t('day.today')}</button>
                    <button aria-label={t('cal.forward')} onClick={() => navigate({ date: stepDate(view, date, 1) })}><Icon name="right" /></button>
                  </div>
                  <button class="rbtn only-cal" aria-label={t('event.new')} onClick={newEvent}><Icon name="plus" /></button>
                </>
              )}
              <FocusCapsule />
              <button class={`rbtn ${panel === 'activity' ? 'on' : ''}`} data-panel-toggle aria-label={t('activity.title')} aria-expanded={panel === 'activity'} onClick={() => setPanel(panel === 'activity' ? null : 'activity')}>
                <Icon name="bell" />
                {pending > 0 && <span class="badge">{cap(pending)}</span>}
              </button>
              <button class="rbtn" aria-label={t('settings.title')} onClick={() => setPanel('settings')}><Icon name="gear" /></button>
              {panel === 'activity' && <ActivityPanel onClose={() => setPanel(null)} />}
              {cal && <CalPopover day={null} place="top:44px;right:0;transform-origin:calc(100% - 60px) 0" />}
            </div>
          </div>
          {cal ? <Calendar /> : focusing ? <FocusMain /> : <ItemsMain />}
        </main>
        {mode === 'items' && <NewItemButton />}
        <nav class="tabbar glass" aria-label={t('mode.label')}>
          <button class={mode === 'items' ? 'on' : ''} aria-pressed={mode === 'items'} onClick={() => setMode('items')}><Icon name="checklist" />{t('mode.items')}</button>
          <button class={cal ? 'on' : ''} aria-pressed={cal} onClick={() => setMode('cal')}><Icon name="cal" />{t('mode.cal')}</button>
          <button class={focusing ? 'on' : ''} aria-pressed={focusing} onClick={() => setMode('focus')}><Icon name="timer" />{t('mode.focus')}</button>
        </nav>
      </div>
      {panel === 'settings' && <Settings onClose={() => setPanel(null)} />}
      <ZoneQuestion />
      <Hud />
    </>
  );
}
