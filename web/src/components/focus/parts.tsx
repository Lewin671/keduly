// Small pieces of the focus timer that appear all over the app: the tomato mark, the draining ring,
// the countdown, the toolbar capsule and the play button of an item.
import type { JSX } from 'preact';
import type { Item } from '../../api/types';
import { t } from '../../i18n';
import { fractionLeft, mmss, secondsLeft, tomatoesNeeded } from '../../lib/focus';
import { focus, focusColor, openTimer, serverNow, sessionTitle, startFocus, workingOn, zen } from '../../state/focus';
import { colorOf, user } from '../../state/store';
import { Icon } from '../Icons';

/** A tomato: filled when earned, an outline when still to do. */
export function Tom({ on = true }: { on?: boolean }): JSX.Element {
  return <svg class={on ? 'tom' : 'tom off'} aria-hidden="true"><use href={on ? '#i-tom' : '#i-tom-o'} /></svg>;
}

/** Tomatoes earned on an item against the number its estimate comes to: "2/4". Nothing before the first one. */
export function TomCount({ item }: { item: Item }): JSX.Element | null {
  if (!item.focus.tomatoes) return null;
  const need = tomatoesNeeded(item.estimate_minutes, user.value!.focus_minutes);
  return <span class="tc"><Tom />{item.focus.tomatoes}{need ? `/${need}` : ''}</span>;
}

const RING = 44;

/** The small ring: what is left of the running session. */
export function Ring({ fraction }: { fraction: number }): JSX.Element {
  return (
    <svg class="ring" viewBox="0 0 18 18" aria-hidden="true">
      <circle class="r0" cx="9" cy="9" r="7" />
      <circle class="r1" cx="9" cy="9" r="7" stroke-dasharray={`${fraction * RING} ${RING}`} />
    </svg>
  );
}

/** Time left of the running session, redrawn as the clock moves. */
export function Countdown(): JSX.Element {
  return <>{mmss(secondsLeft(focus.value!.session!, serverNow()))}</>;
}

/** The running timer in the toolbar, shown wherever the timer page is not. */
export function FocusCapsule(): JSX.Element | null {
  const f = focus.value;
  if (!f || f.state === 'idle' || zen.value) return null;
  const s = f.session!;
  const counting = f.state !== 'over';
  const name = f.state === 'rest' ? t(f.round_done === f.round_size ? 'focus.longRest' : 'focus.rest') : sessionTitle(s);
  return (
    <button class="fcap" style={{ '--c': focusColor(f) }} title={name} onClick={openTimer}>
      <Ring fraction={counting ? fractionLeft(s, serverNow()) : 0} />
      <b>{counting ? <Countdown /> : t('focus.timeUp')}</b>
      <span>{name}</span>
    </button>
  );
}

/** On an item row: the countdown while a tomato runs on it, otherwise a button that starts one. */
export function RowFocus({ item }: { item: Item }): JSX.Element | null {
  if (item.status === 'done' || !item.id) return null;
  if (workingOn(item.id)) {
    return (
      <span class="fgo" style={{ '--c': colorOf(item.project_id) }}>
        <Ring fraction={fractionLeft(focus.value!.session!, serverNow())} />
        <span><Countdown /></span>
      </span>
    );
  }
  return (
    <button class="play" aria-label={t('focus.play', { title: item.title })} title={t('focus.start')} onClick={() => { void startFocus(item.id, item); }}>
      <Icon name="play" />
    </button>
  );
}
