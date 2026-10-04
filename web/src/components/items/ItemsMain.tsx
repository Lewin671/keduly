// The list on screen in items mode, and the ways to add an item to it.
import type { JSX } from 'preact';
import { useEffect } from 'preact/hooks';
import { t } from '../../i18n';
import { resetListState, startDraft } from '../../state/items';
import { route } from '../../state/route';
import { today } from '../../state/store';
import { Icon } from '../Icons';
import { All, Done, Inbox, Matrix, Today, Upcoming } from './lists';
import { ProjectPage } from './ProjectPage';

/** A new item starts where the user is: in the open project, planned for today in Today, else in the inbox. */
function newItem(): void {
  const list = route.value.list;
  startDraft({
    project_id: list.startsWith('p:') ? list.slice(2) : null,
    heading_id: null,
    planned_date: list === 'today' ? today.value : null,
  });
}

function isTyping(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));
}

export function ItemsMain(): JSX.Element {
  const list = route.value.list;

  useEffect(() => resetListState, [list]);
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== 'n' || event.metaKey || event.ctrlKey || event.altKey || isTyping(event.target)) return;
      if (document.querySelector('.scrim, .pop, .panel')) return;
      event.preventDefault();
      newItem();
    };
    document.addEventListener('keydown', key);
    return () => document.removeEventListener('keydown', key);
  }, []);

  return (
    <div class={list === 'matrix' ? 'list wide' : 'list'} id="list">
      {list.startsWith('p:') ? <ProjectPage key={list} id={list.slice(2)} />
        : list === 'inbox' ? <Inbox />
        : list === 'upcoming' ? <Upcoming />
        : list === 'matrix' ? <Matrix />
        : list === 'all' ? <All />
        : list === 'done' ? <Done />
        : <Today />}
    </div>
  );
}

export function NewItemButton(): JSX.Element {
  return <button class="fab" aria-label={t('item.new')} onClick={newItem}><Icon name="plus" /></button>;
}
