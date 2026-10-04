// One item: a row in a list, or, when opened, a card in place that edits it.
import type { JSX } from 'preact';
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { Item, ItemWrite } from '../../api/types';
import { t } from '../../i18n';
import { hhmm, ymd } from '../../lib/dates';
import { isComposing, useEscape } from '../../lib/keys';
import { dayLabel, dueLabel, dur, monthDay, overdueDays, slotLabel, whenLabel } from '../../lib/format';
import { current, draft, NEW, openCard, openItem, saveItem, toggleDone } from '../../state/items';
import { route } from '../../state/route';
import { attempt, projectOf, refresh, today } from '../../state/store';
import { showHud } from '../../state/ui';
import { Icon } from '../Icons';
import { ItemChips } from './ItemChips';

export interface RowOptions {
  /** Show the project and the estimate under the title. */
  sub?: boolean;
  /** Mark items planned for today with a star. */
  star?: boolean;
  /** The day is already in a heading above: show times only. */
  timeOnly?: boolean;
  /** Show the day the item was completed. */
  doneDate?: boolean;
}

const SAVE_DELAY = 800;

function useAutosize(value: string) {
  const ref = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [value]);
  return ref;
}

function RightSide({ item, opts }: { item: Item; opts: RowOptions }): JSX.Element {
  const day = today.value;
  const done = item.status === 'done';
  const parts: JSX.Element[] = [];
  if (done && opts.doneDate && item.completed_at) {
    parts.push(<span>{monthDay(ymd(new Date(item.completed_at)), day)}</span>);
  }
  if (item.due_date && !done) {
    const late = overdueDays(item.due_date, day);
    parts.push(
      <span class={`flag ${late >= 0 ? 'hot' : ''}`}>
        <Icon name="flag" />
        {late > 0 ? t('item.overdue', { n: late }) : dueLabel(item.due_date, item.due_time, day)}
      </span>,
    );
  }
  if (item.suggestion && !done) {
    const start = new Date(item.suggestion.start);
    parts.push(<span class="chip-t">{t('item.pending', { slot: opts.timeOnly ? hhmm(start) : slotLabel(start, day) })}</span>);
  } else if (item.block) {
    const start = new Date(item.block.start);
    parts.push(<span>{opts.timeOnly ? hhmm(start) : whenLabel(start, day)}</span>);
  } else if (item.planned_date && !done && !opts.timeOnly && item.planned_date !== day) {
    parts.push(<span>{dayLabel(item.planned_date, day)}</span>);
  }
  return <div class="rt">{parts}</div>;
}

export function ItemRow({ item: raw, opts = {} }: { item: Item; opts?: RowOptions }): JSX.Element {
  const item = current(raw);
  if (openCard.value === item.id) return <OpenItem key={item.id} item={item} opts={opts} />;
  const done = item.status === 'done';
  const project = projectOf(item.project_id);
  const sub = opts.sub ? [project?.name, item.estimate_minutes ? dur(item.estimate_minutes) : ''].filter(Boolean).join(' · ') : '';
  const starred = opts.star && !done && item.planned_date !== null && item.planned_date <= today.value;
  const open = () => openItem(item.id);
  return (
    <div class={`todo ${done ? 'done' : ''}`}>
      <div class="tr" onClick={event => { if (!(event.target as Element).closest('button')) open(); }}>
        <button class={`chk ${done ? 'on' : ''}`} role="checkbox" aria-checked={done} aria-label={t('item.complete', { title: item.title })} onClick={() => { void toggleDone(item); }} />
        <div class="tm" role="button" tabIndex={0} onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); open(); } }}>
          <div class="tt">
            {item.important && !done && <b class="imp" title={t('item.important')}>!</b>}
            {starred && <Icon name="star" />}
            {item.title}
          </div>
          {sub && <div class="ts">{sub}</div>}
        </div>
        <RightSide item={item} opts={opts} />
      </div>
    </div>
  );
}

/** The card of an item that does not exist yet. It is discarded if closed without a title. */
export function DraftRow(): JSX.Element | null {
  const d = draft.value;
  if (!d || openCard.value !== NEW) return null;
  const item: Item = d.item ? current(d.item) : {
    id: '', title: '', notes: '', estimate_minutes: null, evening: false, due_date: null, due_time: null, important: false,
    status: 'open', completed_at: null, position: 0, block: null, suggestion: null,
    created_by: { kind: 'user', name: '' }, created_at: '', updated_at: '', ...d.context,
  };
  return <OpenItem key={NEW} item={item} opts={{}} isDraft />;
}

function OpenItem({ item, opts, isDraft = false }: { item: Item; opts: RowOptions; isDraft?: boolean }): JSX.Element {
  const [title, setTitle] = useState(item.title);
  const [notes, setNotes] = useState(item.notes);
  const root = useRef<HTMLDivElement>(null);
  const titleRef = useAutosize(title);
  const notesRef = useAutosize(notes);
  // The newest values, for saves that run from timers and from unmount.
  const live = useRef({ item, title, notes, saved: { title: item.title, notes: item.notes } });
  live.current.item = item;
  live.current.title = title;
  live.current.notes = notes;
  const creating = useRef<Promise<Item | undefined> | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Closing the card clears the draft before the last save runs, so its context is kept here.
  const context = useRef(draft.value?.context);
  const done = item.status === 'done';

  /** The item on the server, created first when this is still a draft. `undefined` without a title. */
  const ensure = (): Promise<Item | undefined> => {
    const now = live.current;
    if (now.item.id) return Promise.resolve(now.item);
    if (creating.current) return creating.current;
    const text = now.title.trim();
    if (!text) {
      titleRef.current?.focus();
      return Promise.resolve(undefined);
    }
    now.saved = { title: text, notes: now.notes };
    creating.current = attempt(api.createItem({ title: text, notes: now.notes, ...context.current })).then(created => {
      // Only while this draft is still the one on screen.
      const d = draft.value;
      if (created && d && d.context === context.current) draft.value = { ...d, item: created };
      if (!created) creating.current = null;
      void refresh();
      return created;
    });
    return creating.current;
  };

  const flush = async (): Promise<void> => {
    clearTimeout(timer.current);
    const now = live.current;
    const text = now.title.trim();
    if (!now.item.id) {
      if (text) await ensure();
      return;
    }
    const body: ItemWrite = {};
    if (text && text !== now.saved.title) body.title = text;
    if (now.notes !== now.saved.notes) body.notes = now.notes;
    if (!Object.keys(body).length) return;
    now.saved = { title: body.title ?? now.saved.title, notes: now.notes };
    await saveItem(api.updateItem(now.item.id, body));
  };

  const edited = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => { void flush(); }, SAVE_DELAY);
  };

  const close = () => {
    const d = draft.value;
    const filed = isDraft && (d?.item || live.current.title.trim()) && !d?.context.project_id && !d?.context.planned_date;
    if (filed && route.value.list !== 'inbox') showHud(t('item.addedToInbox'));
    openItem(null);
  };

  useEscape(close);
  useEffect(() => {
    if (isDraft || !item.title) titleRef.current?.focus();
    return () => { void flush(); };
  }, []);

  // A change made elsewhere shows up in a field that is neither focused nor being edited.
  useEffect(() => {
    const now = live.current;
    const sync = (field: 'title' | 'notes', el: HTMLTextAreaElement | null, set: (text: string) => void) => {
      if (now[field] !== now.saved[field] || document.activeElement === el) return;
      now.saved[field] = item[field];
      set(item[field]);
    };
    sync('title', titleRef.current, setTitle);
    sync('notes', notesRef.current, setNotes);
  }, [item.title, item.notes]);

  // A click anywhere else closes the card. Clicks, not presses: closing moves the rows below.
  useEffect(() => {
    const click = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Element) || !target.isConnected) return;
      if (root.current?.contains(target) || target.closest('.pop, .scrim, .hud, .fab')) return;
      close();
    };
    document.addEventListener('click', click, true);
    return () => document.removeEventListener('click', click, true);
  }, []);

  return (
    <div ref={root} class={`todo open ${done ? 'done' : ''}`}>
      <div class="tr" onClick={event => { if (!(event.target as Element).closest('button, textarea')) close(); }}>
        <button
          class={`chk ${done ? 'on' : ''}`} role="checkbox" aria-checked={done} disabled={!item.id}
          aria-label={t('item.complete', { title: item.title })} onClick={() => { void toggleDone(item); }}
        />
        <div class="tm">
          <textarea
            ref={titleRef} class="tt ed" rows={1} value={title} placeholder={t('item.newTitle')} aria-label={t('item.title')}
            onInput={event => { setTitle(event.currentTarget.value.replace(/\n/g, ' ')); edited(); }}
            onBlur={() => { void flush(); }}
            onKeyDown={event => {
              if (event.key !== 'Enter' || isComposing(event)) return;
              event.preventDefault();
              close();
            }}
          />
        </div>
        <RightSide item={item} opts={opts} />
      </div>
      <div class="card">
        <textarea
          ref={notesRef} class="cn ed" rows={1} value={notes} placeholder={t('item.notes')} aria-label={t('item.notes')}
          onInput={event => { setNotes(event.currentTarget.value); edited(); }}
          onBlur={() => { void flush(); }}
        />
        <ItemChips item={item} ensure={ensure} onDeleted={() => openItem(null)} />
      </div>
    </div>
  );
}
