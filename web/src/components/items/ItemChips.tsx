// The chips at the bottom of an item card. Each opens a small popover that changes one thing.
import type { JSX } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { FreeSlot, Item, ItemWrite } from '../../api/types';
import { t } from '../../i18n';
import { atLocal, hhmm, snap, toUtc, ymd, wall } from '../../lib/dates';
import { dayLabel, dueLabel, dur, slotLabel, span } from '../../lib/format';
import { openTimer, startFocus, workingOn } from '../../state/focus';
import { removeItem, saveItem, toggleImportant } from '../../state/items';
import { activeProjects, areas, colorOf, headings, now, projectOf, today } from '../../state/store';
import { decide, describe, timeRange } from '../../state/suggestions';
import { Icon } from '../Icons';
import { Popover } from '../Popover';

type Chip = 'when' | 'estimate' | 'due' | 'project' | 'delete';

interface Props {
  item: Item;
  /** Resolves to the saved item, creating a draft first; `undefined` when it has no title yet. */
  ensure: () => Promise<Item | undefined>;
  onDeleted: () => void;
}

export const DURATIONS = [15, 30, 60, 90, 120];

export function ItemChips({ item, ensure, onDeleted }: Props): JSX.Element {
  const [chip, setChip] = useState<Chip | null>(null);
  const day = today.value;
  const project = projectOf(item.project_id);
  const toggle = (name: Chip) => setChip(chip === name ? null : name);
  const close = () => setChip(null);

  /** Applies a change to the saved item and closes the popover. */
  const apply: Apply = async (work, keepOpen = false) => {
    const saved = await ensure();
    if (!saved) return;
    if (!keepOpen) close();
    await saveItem(work(saved));
  };
  const patch: Patch = (body, keepOpen) => apply(saved => api.updateItem(saved.id, body), keepOpen);

  const whenText = item.block ? `${dayLabel(ymd(wall(item.block.start)), day)} ${timeRange(item.block.start, item.block.end)}`
    : item.suggestion ? t('item.pending', { slot: slotLabel(wall(item.suggestion.start), day) })
    : item.planned_date ? (item.evening && item.planned_date === day ? t('when.tonight') : dayLabel(item.planned_date, day))
    : t('when.none');

  return (
    <>
      {item.suggestion && (
        <div class="sg">
          <div><b>{item.suggestion.actor.name}</b> {t('suggest.line', { what: describe({ kind: 'schedule_item', start: item.suggestion.start, end: item.suggestion.end, event: null }), why: item.suggestion.reason })}</div>
          <button class="sb" onClick={() => { void decide(item.suggestion!.id, item.title, false); }}>{t('suggest.reject')}</button>
          <button class="sb go" onClick={() => { void decide(item.suggestion!.id, item.title, true); }}>{t('suggest.accept')}</button>
        </div>
      )}
      <div class="cm">
        <span class="anch">
          <button class="ck" aria-expanded={chip === 'when'} onClick={() => toggle('when')}><Icon name="cal" />{whenText}</button>
          {chip === 'when' && <WhenPop item={item} apply={apply} onClose={close} />}
        </span>
        <span class="anch">
          <button class="ck" aria-expanded={chip === 'estimate'} onClick={() => toggle('estimate')}>
            <Icon name="clock" />{item.estimate_minutes ? dur(item.estimate_minutes) : t('estimate.none')}{item.focus.minutes > 0 ? ` · ${t('focus.used', { dur: span(item.focus.minutes) })}` : ''}
          </button>
          {chip === 'estimate' && <EstimatePop item={item} patch={patch} onClose={close} />}
        </span>
        {item.id && item.status === 'open' && (workingOn(item.id)
          ? <button class="ck go" onClick={openTimer}><Icon name="timer" />{t('focus.back')}</button>
          : <button class="ck go" onClick={() => { void startFocus(item.id, item); }}><Icon name="play" />{t('focus.start')}</button>)}
        <span class="anch">
          <button class="ck" aria-expanded={chip === 'due'} onClick={() => toggle('due')}>
            <Icon name="flag" />{item.due_date ? t('due.label', { due: dueLabel(item.due_date, item.due_time, day) }) : t('due.none')}
          </button>
          {chip === 'due' && <DuePop item={item} patch={patch} onClose={close} />}
        </span>
        <button
          class={`ck ${item.important ? 'hot' : ''}`} aria-pressed={item.important}
          onClick={() => { void ensure().then(saved => { if (saved) void toggleImportant({ ...saved, important: item.important }); }); }}
        >
          <b>!</b>{item.important ? t('item.important') : t('item.markImportant')}
        </button>
        <span class="anch">
          <button class="ck" aria-expanded={chip === 'project'} onClick={() => toggle('project')}>
            <i style={{ '--c': colorOf(item.project_id) }} />{project ? project.name : t('nav.inbox')}
          </button>
          {chip === 'project' && <ProjectPop item={item} patch={patch} onClose={close} />}
        </span>
        <span class="anch">
          <button class="ck" aria-expanded={chip === 'delete'} aria-label={t('common.delete')} onClick={() => toggle('delete')}><Icon name="trash" /></button>
          {chip === 'delete' && (
            <Popover onClose={close} class="narrow" label={t('common.delete')}>
              <div class="pt">{t('item.deleteAsk')}</div>
              <div class="pa">
                <button class="pbtn" onClick={close}>{t('common.cancel')}</button>
                <button
                  class="pbtn danger"
                  onClick={() => {
                    close();
                    if (!item.id) onDeleted();
                    else void removeItem(item.id).then(ok => { if (ok) onDeleted(); });
                  }}
                >{t('common.delete')}</button>
              </div>
            </Popover>
          )}
        </span>
      </div>
    </>
  );
}

type Apply = (work: (saved: Item) => Promise<Item>, keepOpen?: boolean) => Promise<void>;
type Patch = (body: ItemWrite, keepOpen?: boolean) => Promise<void>;

/** The next quarter of an hour from now, as a starting time to offer. */
function nextSlot(): string {
  const d = new Date(now.value);
  d.setMinutes(Math.min(snap(d.getMinutes() + 8), 60), 0, 0);
  return hhmm(d);
}

function WhenPop({ item, apply, onClose }: { item: Item; apply: Apply; onClose: () => void }): JSX.Element {
  const slot = item.block ?? item.suggestion;
  const [date, setDate] = useState(slot ? ymd(wall(slot.start)) : (item.planned_date ?? today.value));
  const [time, setTime] = useState(slot ? hhmm(wall(slot.start)) : nextSlot());
  const [minutes, setMinutes] = useState(
    item.block ? Math.max(15, Math.round((Date.parse(item.block.end) - Date.parse(item.block.start)) / 60_000)) : (item.estimate_minutes ?? 60),
  );
  const [free, setFree] = useState<FreeSlot[]>([]);

  // Free slots are a convenience; a failure only means none are offered.
  useEffect(() => {
    let live = true;
    api.getFree(date, minutes).then(slots => { if (live) setFree(slots); }, () => { if (live) setFree([]); });
    return () => { live = false; };
  }, [date, minutes]);

  const unblock = (saved: Item) => (saved.block ? api.unscheduleItem(saved.id) : Promise.resolve(saved));
  const schedule = () => apply(saved => {
    const start = atLocal(date, time);
    return api.scheduleItem(saved.id, toUtc(start), toUtc(new Date(start.getTime() + minutes * 60_000)));
  });
  const plan = (body: ItemWrite) => apply(saved => unblock(saved).then(() => api.updateItem(saved.id, body)));

  return (
    <Popover onClose={onClose} label={t('when.title')}>
      <form class="form" onSubmit={event => { event.preventDefault(); if (date && time) void schedule(); }}>
        <div class="frow">
          <input class="fld grow" type="date" required value={date} aria-label={t('field.date')} onInput={event => setDate(event.currentTarget.value)} />
          <input class="fld" type="time" required step={900} value={time} aria-label={t('field.start')} onInput={event => setTime(event.currentTarget.value)} />
        </div>
        <div class="seg small" role="group" aria-label={t('field.duration')}>
          {DURATIONS.map(m => (
            <button type="button" class={m === minutes ? 'on' : ''} aria-pressed={m === minutes} onClick={() => setMinutes(m)}>{dur(m)}</button>
          ))}
        </div>
        {free.length > 0 && (
          <div class="slots">
            <span>{t('when.free')}</span>
            {free.slice(0, 4).map(s => (
              <button type="button" class="tb" onClick={() => setTime(hhmm(wall(s.start)))}>{hhmm(wall(s.start))}</button>
            ))}
          </div>
        )}
        <div class="pa">
          <button type="submit" class="pbtn go">{t('when.schedule')}</button>
        </div>
        <div class="links">
          <button type="button" class="tb" onClick={() => { if (date) void plan({ planned_date: date, evening: false }); }}>{t('when.dateOnly')}</button>
          <button type="button" class="tb" onClick={() => { void plan({ planned_date: today.value, evening: true }); }}>{t('when.tonight')}</button>
          {(item.block || item.planned_date) && (
            <button type="button" class="tb danger" onClick={() => { void plan({ planned_date: null, evening: false }); }}>{t('when.clear')}</button>
          )}
        </div>
      </form>
    </Popover>
  );
}

function EstimatePop({ item, patch, onClose }: { item: Item; patch: Patch; onClose: () => void }): JSX.Element {
  const [custom, setCustom] = useState(item.estimate_minutes && !DURATIONS.includes(item.estimate_minutes) ? String(item.estimate_minutes) : '');
  const minutes = Number(custom);
  return (
    <Popover onClose={onClose} label={t('estimate.title')}>
      <form class="form" onSubmit={event => { event.preventDefault(); if (Number.isInteger(minutes) && minutes > 0) void patch({ estimate_minutes: minutes }); }}>
        <div class="seg small" role="group" aria-label={t('estimate.title')}>
          {DURATIONS.map(m => (
            <button type="button" class={m === item.estimate_minutes ? 'on' : ''} aria-pressed={m === item.estimate_minutes} onClick={() => { void patch({ estimate_minutes: m }); }}>{dur(m)}</button>
          ))}
        </div>
        <div class="frow">
          <input class="fld grow" type="number" min={1} max={1440} step={1} inputMode="numeric" value={custom} placeholder={t('estimate.custom')} aria-label={t('estimate.custom')} onInput={event => setCustom(event.currentTarget.value)} />
          <button type="submit" class="sb go">{t('common.ok')}</button>
        </div>
        {item.estimate_minutes !== null && (
          <div class="links"><button type="button" class="tb danger" onClick={() => { void patch({ estimate_minutes: null }); }}>{t('common.clear')}</button></div>
        )}
      </form>
    </Popover>
  );
}

function DuePop({ item, patch, onClose }: { item: Item; patch: Patch; onClose: () => void }): JSX.Element {
  const [date, setDate] = useState(item.due_date ?? today.value);
  const [time, setTime] = useState(item.due_time ?? '');
  return (
    <Popover onClose={onClose} label={t('due.title')}>
      <form class="form" onSubmit={event => { event.preventDefault(); if (date) void patch({ due_date: date, due_time: time || null }); }}>
        <div class="frow">
          <input class="fld grow" type="date" required value={date} aria-label={t('due.date')} onInput={event => setDate(event.currentTarget.value)} />
          <input class="fld" type="time" value={time} aria-label={t('due.time')} onInput={event => setTime(event.currentTarget.value)} />
        </div>
        <div class="pa">
          {item.due_date && <button type="button" class="pbtn" onClick={() => { void patch({ due_date: null, due_time: null }); }}>{t('common.clear')}</button>}
          <button type="submit" class="pbtn go">{t('common.ok')}</button>
        </div>
      </form>
    </Popover>
  );
}

function ProjectPop({ item, patch, onClose }: { item: Item; patch: Patch; onClose: () => void }): JSX.Element {
  const list = activeProjects.value;
  const mine = headings.value.filter(h => h.project_id === item.project_id);
  const loose = list.filter(p => !areas.value.some(a => a.id === p.area_id));
  return (
    <Popover onClose={onClose} label={t('project.pick')}>
      <div class="form">
        <label class="lbl">{t('field.project')}
          <select class="fld" value={item.project_id ?? ''} onChange={event => { void patch({ project_id: event.currentTarget.value || null, heading_id: null }, true); }}>
            <option value="">{t('nav.inbox')}</option>
            {loose.map(p => <option value={p.id}>{p.name}</option>)}
            {areas.value.filter(a => list.some(p => p.area_id === a.id)).map(a => (
              <optgroup label={a.name}>
                {list.filter(p => p.area_id === a.id).map(p => <option value={p.id}>{p.name}</option>)}
              </optgroup>
            ))}
          </select>
        </label>
        {mine.length > 0 && (
          <label class="lbl">{t('field.heading')}
            <select class="fld" value={item.heading_id ?? ''} onChange={event => { void patch({ heading_id: event.currentTarget.value || null }); }}>
              <option value="">{t('heading.none')}</option>
              {mine.map(h => <option value={h.id}>{h.name}</option>)}
            </select>
          </label>
        )}
      </div>
    </Popover>
  );
}
