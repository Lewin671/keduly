// What a session was for: a project, then one of its items, or a title of its own.
import type { JSX } from 'preact';
import { useEffect, useRef } from 'preact/hooks';
import * as api from '../../api/client';
import type { FocusSession, Item } from '../../api/types';
import { t } from '../../i18n';
import { fileSession, freeTitle, todayItems } from '../../state/focus';
import { current } from '../../state/items';
import { useResource } from '../../state/resource';
import { projectOf } from '../../state/store';
import { ProjectOptions } from '../items/parts';
import { Popover } from '../Popover';

const NONE: Item[] = [];

/** Each change applies at once. Without a project the items offered are today's. */
export function SessionPop({ session: s, onClose, place }: { session: FocusSession; onClose: () => void; /** Where it hangs when it has no anchor of its own. */ place?: string }): JSX.Element {
  const projectId = s.project_id;
  const inProject = useResource(`session-items:${projectId ?? ''}`, () => (projectId ? api.listItems({ project_id: projectId, status: 'open', limit: 200 }).then(page => page.items) : Promise.resolve(NONE)));
  const offered = (projectId ? inProject.data ?? NONE : todayItems.value ?? NONE).map(current);

  // The title is saved when the field is left, and when the popover closes with it still in the field.
  const typed = useRef(s.title);
  const saved = useRef(s.title);
  const free = useRef(!s.item_id);
  free.current = !s.item_id;
  const saveTitle = () => {
    const title = typed.current.trim();
    if (!free.current || title === saved.current) return;
    saved.current = title;
    void fileSession(s.id, { title });
  };
  // What the server says replaces what was typed, e.g. once the session is taken off an item.
  useEffect(() => { typed.current = saved.current = s.title; }, [s.title]);
  const leave = useRef(saveTitle);
  leave.current = saveTitle;
  useEffect(() => () => leave.current(), []);

  return (
    <Popover onClose={onClose} label={t('session.pick')} {...(place ? { style: place, within: 'self' as const } : {})}>
      <div class="form">
        <label class="lbl">{t('field.project')}
          <select class="fld" value={projectId ?? ''} onChange={event => { void fileSession(s.id, { item_id: null, project_id: event.currentTarget.value || null }); }}>
            <option value="">{t('stats.noProject')}</option>
            <ProjectOptions also={projectOf(projectId)} />
          </select>
        </label>
        <label class="lbl">{t('session.item')}
          <select
            class="fld" value={s.item_id ?? ''}
            onChange={event => {
              const id = event.currentTarget.value;
              // Taken off its item, the session stays in the item's project.
              void fileSession(s.id, id ? { item_id: id } : { item_id: null, project_id: projectId }, offered.find(i => i.id === id));
            }}
          >
            <option value="">{t('session.noItem')}</option>
            {s.item_id && !offered.some(i => i.id === s.item_id) && <option value={s.item_id}>{s.title}</option>}
            {offered.map(i => <option value={i.id}>{i.title}</option>)}
          </select>
        </label>
        {!s.item_id && (
          <label class="lbl">{t('session.what')}
            <input
              class="fld" value={s.title} placeholder={freeTitle()} maxLength={500}
              onInput={event => { typed.current = event.currentTarget.value; }}
              onChange={saveTitle}
              onKeyDown={event => { if (event.key === 'Enter') event.currentTarget.blur(); }}
            />
          </label>
        )}
      </div>
    </Popover>
  );
}
