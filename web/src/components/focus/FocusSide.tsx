// The sidebar in focus mode: timer, statistics, and what the next tomato can be for.
import type { JSX } from 'preact';
import type { Item } from '../../api/types';
import { t } from '../../i18n';
import { tomatoesNeeded } from '../../lib/focus';
import { focus, openTimer, pick, pickedItem, picks } from '../../state/focus';
import { navigate, route } from '../../state/route';
import { colorOf, user } from '../../state/store';
import { Icon } from '../Icons';
import { Tom, TomCount } from './parts';

function Choice({ item, chosen }: { item: Item | null; chosen: boolean }): JSX.Element {
  const title = item ? item.title : t('focus.free');
  const need = item ? tomatoesNeeded(item.estimate_minutes, user.value!.focus_minutes) : 0;
  // While a timer is active the choice cannot change: the click only leads back to the timer.
  const choose = () => {
    if (focus.value?.state === 'idle') pick.value = item ? item.id : null;
    openTimer();
  };
  return (
    <button class={chosen ? 'nv sel' : 'nv'} title={title} aria-pressed={chosen} onClick={choose}>
      <i class={chosen ? 'fdot' : 'fdot o'} style={{ '--c': colorOf(item?.project_id) }} />
      <span class="nl">{title}</span>
      {item && (item.focus.tomatoes > 0 ? <span class="n"><TomCount item={item} /></span> : need > 0 ? <span class="n"><Tom on={false} />{need}</span> : null)}
    </button>
  );
}

export function FocusSide(): JSX.Element {
  const view = route.value.focus;
  const f = focus.value;
  // The marked choice is what the timer is on, or what the next tomato will be for.
  const active = f && (f.state === 'work' || f.state === 'over') ? f.session!.item_id : (pickedItem()?.id ?? null);
  const marked = view === 'timer' && f !== null;
  return (
    <div id="side-focus">
      <div class="nav">
        <button class={`nv ${view === 'timer' ? 'on' : ''}`} style={{ '--c': 'var(--tomato)' }} aria-current={view === 'timer' ? 'page' : undefined} onClick={openTimer}>
          <Icon name="timer" /><span class="nl">{t('focus.timer')}</span>
        </button>
        <button class={`nv ${view === 'stats' ? 'on' : ''}`} style={{ '--c': 'var(--blue)' }} aria-current={view === 'stats' ? 'page' : undefined} onClick={() => { navigate({ mode: 'focus', focus: 'stats' }); scrollTo(0, 0); }}>
          <Icon name="chart" /><span class="nl">{t('focus.stats')}</span>
        </button>
        <div class="area">{t('focus.today')}</div>
        {picks().map(item => <Choice key={item.id} item={item} chosen={marked && active === item.id} />)}
        <Choice item={null} chosen={marked && active === null} />
      </div>
    </div>
  );
}
