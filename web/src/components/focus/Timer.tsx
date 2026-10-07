// The timer page: one large dial, what the tomato is for, the round, and what can be done now.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import type { Focus, Item } from '../../api/types';
import { t } from '../../i18n';
import { hhmm, wall } from '../../lib/dates';
import { fractionLeft, mmss } from '../../lib/focus';
import { dur, span } from '../../lib/format';
import { againFocus, finishFocus, focus, focusColor, freeTitle, knownItem, pickedItem, restFocus, serverNow, startFocus, stopFocus } from '../../state/focus';
import { modeBehindFocus, navigate } from '../../state/route';
import { colorOf, projectOf, user } from '../../state/store';
import { Icon } from '../Icons';
import { Skeleton } from '../items/parts';
import { Countdown, Tom, TomCount } from './parts';
import { SessionPop } from './SessionPop';

const DIAL = 879.6;

/** What the page is about: the running session's item, or the one picked for the next tomato. */
interface Subject {
  title: string;
  projectId: string | null;
  /** `null` for free focus. */
  itemId: string | null;
  item: Item | undefined;
}

function subjectOf(f: Focus): Subject {
  if (f.state === 'work' || f.state === 'over') {
    const s = f.session!;
    return { title: s.title || freeTitle(), projectId: s.project_id, itemId: s.item_id, item: knownItem(s.item_id) };
  }
  const item = pickedItem();
  return item ? { title: item.title, projectId: item.project_id, itemId: item.id, item } : { title: freeTitle(), projectId: null, itemId: null, item: undefined };
}

function Round({ f }: { f: Focus }): JSX.Element {
  return (
    <div class="round">
      {Array.from({ length: f.round_size }, (_, k) => <Tom key={k} on={k < f.round_done} />)}
      <span>{f.round_done === f.round_size ? t('focus.roundDone') : t('focus.roundLeft', { n: f.round_size - f.round_done })}</span>
    </div>
  );
}

function Actions({ f, subject }: { f: Focus; subject: Subject }): JSX.Element {
  // A tomato on an item can end with the item being done; free focus has nothing to tick.
  const finish = subject.itemId && subject.item?.status !== 'done' ? <button class="zb" onClick={() => { void finishFocus(); }}>{t('focus.finish')}</button> : null;
  switch (f.state) {
    case 'idle':
      return <button class="zb go" onClick={() => { void startFocus(subject.itemId, subject.item); }}>{t('focus.start')}</button>;
    case 'work':
      return <><button class="zb" onClick={() => { void stopFocus(); }}>{t('focus.giveUp')}</button>{finish}</>;
    case 'over':
      return (
        <>
          {finish}
          <button class="zb" onClick={() => { void againFocus(); }}>{t('focus.again')}</button>
          <button class="zb go" onClick={() => { void restFocus(); }}>
            {t(f.round_done === f.round_size ? 'focus.takeLongRest' : 'focus.takeRest', { n: f.rest_minutes })}
          </button>
        </>
      );
    case 'rest':
      return <button class="zb" onClick={() => { void stopFocus(); }}>{t('focus.skipRest')}</button>;
  }
}

export function Timer(): JSX.Element {
  const f = focus.value;
  if (!f) return <div class="zen"><Skeleton /></div>;
  const me = user.value!;
  const subject = subjectOf(f);
  // Free focus can be named and filed while it runs and when it has just been earned.
  const free = (f.state === 'work' || f.state === 'over') && !f.session!.item_id ? f.session! : null;
  const [filing, setFiling] = useState<string | null>(null);
  const long = f.round_done === f.round_size;
  const label =
    f.state === 'over' ? t('focus.earned', { n: f.tomatoes_today })
    : f.state === 'rest' ? t(long ? 'focus.longRest' : 'focus.rest')
    : t('focus.nth', { n: f.tomatoes_today + 1 });
  // A finished tomato closes the ring in tomato red.
  const color = f.state === 'over' ? 'var(--tomato)' : f.state === 'rest' ? focusColor(f) : colorOf(subject.projectId);
  const fraction = f.state === 'work' || f.state === 'rest' ? fractionLeft(f.session!, serverNow()) : 1;
  const project = projectOf(subject.projectId);
  const facts: Array<JSX.Element | string> = [];
  if (subject.itemId) {
    facts.push(project ? project.name : t('nav.inbox'));
    if (subject.item?.estimate_minutes) facts.push(dur(subject.item.estimate_minutes));
    if (subject.item && subject.item.focus.tomatoes > 0) facts.push(<TomCount item={subject.item} />);
  } else {
    facts.push(project ? project.name : t('focus.freeNote'));
    if (free) {
      facts.push(
        <span class="anch">
          <button class="zlink" aria-expanded={filing === free.id} onClick={() => setFiling(filing === free.id ? null : free.id)}>
            {t(free.title || free.project_id ? 'focus.refile' : 'focus.file')}
          </button>
          {filing === free.id && <SessionPop session={free} onClose={() => setFiling(null)} />}
        </span>,
      );
    }
  }

  return (
    <div class="zen" style={{ '--c': color }}>
      {f.state !== 'idle' && (
        <button class="zback" onClick={() => { navigate({ mode: modeBehindFocus() }); scrollTo(0, 0); }}><Icon name="left" />{t('focus.collapse')}</button>
      )}
      <div class="zlab">{label}</div>
      <div class="dial">
        <svg viewBox="0 0 300 300" aria-hidden="true">
          <circle class="r0" cx="150" cy="150" r="140" />
          <circle class="r1" cx="150" cy="150" r="140" stroke-dasharray={`${fraction * DIAL} ${DIAL}`} />
        </svg>
        <div class="dt">
          {f.state === 'over' ? <Tom /> : (
            <>
              <b role="timer">{f.state === 'idle' ? mmss(me.focus_minutes * 60) : <Countdown />}</b>
              <span>{f.state === 'work' ? t('focus.until', { time: hhmm(wall(f.session!.end)) }) : f.state === 'rest' ? t('focus.resting') : t('focus.idle')}</span>
            </>
          )}
        </div>
      </div>
      <div class="zt" title={subject.title}><i style={{ '--c': colorOf(subject.projectId) }} />{subject.title}</div>
      <div class="zs">{facts.map((fact, k) => <>{k > 0 && ' · '}{fact}</>)}</div>
      <Round f={f} />
      <div class="za"><Actions f={f} subject={subject} /></div>
      {f.state === 'idle' && f.minutes_today > 0 && <div class="zf">{t('focus.todayTotal')} <Tom />{f.tomatoes_today} · {span(f.minutes_today)}</div>}
    </div>
  );
}
