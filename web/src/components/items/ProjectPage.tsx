// A project: notes, what is coming up on the calendar, then items under their headings.
import { signal } from '@preact/signals';
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { Heading, Item, Project } from '../../api/types';
import { t } from '../../i18n';
import { ymd } from '../../lib/dates';
import { dayLabel } from '../../lib/format';
import { current, draft, fresh, startDraft } from '../../state/items';
import { useResource } from '../../state/resource';
import { navigate } from '../../state/route';
import { colorOf, headings as allHeadings, projectOf, today, write } from '../../state/store';
import { showHud } from '../../state/ui';
import { Icon, Pie } from '../Icons';
import { InlineText } from '../InlineText';
import { Confirm, Popover } from '../Popover';
import { ProjectDialog } from '../ProjectDialog';
import { DraftRow } from './ItemRow';
import { Blank, EventsStrip, eventTime, ListHead, MoreButton, Rows, Skeleton } from './parts';
import { usePaged } from './usePaged';

/** Headings folded during this session. */
const folded = signal<ReadonlySet<string>>(new Set());

function toggleFold(id: string): void {
  const next = new Set(folded.value);
  if (!next.delete(id)) next.add(id);
  folded.value = next;
}

function ProjectMenu({ project }: { project: Project }): JSX.Element {
  const [open, setOpen] = useState<'menu' | 'edit' | 'delete' | null>(null);
  const close = () => setOpen(null);
  const archive = async () => {
    close();
    if (!(await write(api.updateProject(project.id, { archived: true })))) return;
    navigate({ mode: 'items', list: 'today' });
    showHud(t('project.archived', { name: project.name }), () => { void write(api.updateProject(project.id, { archived: false })); });
  };
  const remove = async () => {
    await write(api.deleteProject(project.id));
    navigate({ mode: 'items', list: 'today' });
  };
  return (
    <span class="anch">
      <button class="mbtn" aria-label={t('project.menu')} aria-expanded={open === 'menu'} onClick={() => setOpen(open === 'menu' ? null : 'menu')}><Icon name="more" /></button>
      {open === 'menu' && (
        <Popover onClose={close} class="menu right" label={t('project.menu')}>
          <button onClick={() => setOpen('edit')}>{t('project.edit')}</button>
          <button onClick={() => { void archive(); }}>{t('project.archive')}</button>
          <button class="danger" onClick={() => setOpen('delete')}>{t('project.delete')}</button>
        </Popover>
      )}
      {open === 'edit' && <ProjectDialog project={project} onClose={close} />}
      {open === 'delete' && (
        <Confirm title={t('project.deleteAsk', { name: project.name })} text={t('project.deleteText')} action={t('common.delete')} onConfirm={() => { void remove(); }} onClose={close} />
      )}
    </span>
  );
}

function HeadingSection({ heading, items }: { heading: Heading; items: Item[] }): JSX.Element {
  const [open, setOpen] = useState<'menu' | 'rename' | 'delete' | null>(null);
  const shut = folded.value.has(heading.id);
  const close = () => setOpen(null);
  const d = draft.value;
  return (
    <>
      <div class="sec-w">
        {open === 'rename' ? (
          <div class="sec"><InlineText class="sec-ed" value={heading.name} label={t('heading.rename')} autoFocus onSave={name => { void write(api.updateHeading(heading.id, { name })); }} onDone={close} /></div>
        ) : (
          <button class={`sec fold ${shut ? 'shut' : ''}`} aria-expanded={!shut} onClick={() => toggleFold(heading.id)}>
            <Icon name="right" class="chev" />
            <span>{heading.name}</span>
            <em>{items.filter(i => current(i).status === 'open').length}</em>
          </button>
        )}
        <span class="anch">
          <button class="mbtn" aria-label={t('heading.menu')} aria-expanded={open === 'menu'} onClick={() => setOpen(open === 'menu' ? null : 'menu')}><Icon name="more" /></button>
          {open === 'menu' && (
            <Popover onClose={close} class="menu right" label={t('heading.menu')}>
              <button onClick={() => {
                close();
                if (shut) toggleFold(heading.id);
                startDraft({ project_id: heading.project_id, heading_id: heading.id, planned_date: null });
              }}>{t('heading.addItem')}</button>
              <button onClick={() => setOpen('rename')}>{t('common.rename')}</button>
              <button class="danger" onClick={() => setOpen('delete')}>{t('heading.delete')}</button>
            </Popover>
          )}
        </span>
      </div>
      {open === 'delete' && (
        <Confirm title={t('heading.deleteAsk', { name: heading.name })} text={t('heading.deleteText')} action={t('common.delete')} onConfirm={() => { void write(api.deleteHeading(heading.id)); }} onClose={close} />
      )}
      {!shut && (
        <>
          {d?.context.heading_id === heading.id && <DraftRow />}
          <Rows items={items} opts={{ star: true }} />
        </>
      )}
    </>
  );
}

function NewHeading({ projectId }: { projectId: string }): JSX.Element {
  const [adding, setAdding] = useState(false);
  if (!adding) return <button class="morebtn flat" onClick={() => setAdding(true)}>{t('heading.new')}</button>;
  return (
    <div class="sec">
      <InlineText class="sec-ed" value="" placeholder={t('heading.name')} label={t('heading.name')} autoFocus onSave={name => { void write(api.createHeading(projectId, name)); }} onDone={() => setAdding(false)} />
    </div>
  );
}

/** Open items of a project are fetched whole: headings need all of them to group. */
const OPEN_BATCH = 200;

export function ProjectPage({ id }: { id: string }): JSX.Element {
  const project = projectOf(id);
  const detail = useResource(`project:${id}`, () => api.getProject(id));
  const open = usePaged({ project_id: id }, { first: OPEN_BATCH, step: OPEN_BATCH });
  const [showDone, setShowDone] = useState(false);
  const logged = usePaged({ project_id: id, status: 'done' }, { first: 10, step: 30, enabled: showDone });

  if (!project) return <Blank title={t('project.missing')} tip={t('project.missingTip')} />;
  const color = colorOf(id);
  const day = today.value;
  const top = (
    <>
      <ListHead icon={<Pie done={project.done_count} open={project.open_count} color={color} />} name={
        <InlineText class="h2" value={project.name} label={t('project.name')} onSave={name => { void write(api.updateProject(id, { name })); }} />
      } color={color}>
        <ProjectMenu project={project} />
      </ListHead>
      <InlineText class="notes" multiline allowEmpty value={project.notes} placeholder={t('item.notes')} label={t('item.notes')} onSave={notes => { void write(api.updateProject(id, { notes })); }} />
    </>
  );
  if (!open.items) return <>{top}<Skeleton /></>;

  const heads = detail.data?.headings ?? allHeadings.value.filter(h => h.project_id === id);
  const known = new Set(heads.map(h => h.id));
  const d = draft.value;
  // Finished during this visit: still listed among the open items, so not counted as filed away.
  const justDone = Object.values(fresh.value).filter(i => i.project_id === id);
  const loggedCount = Math.max(project.done_count - justDone.length, 0);
  const openCount = project.open_count;
  const gaps = detail.data?.unplanned_count ?? 0;
  const loose = open.items.filter(i => !i.heading_id || !known.has(i.heading_id));
  const empty = !open.items.length && !justDone.length && !heads.length && !loggedCount;

  return (
    <>
      {top}
      {!empty && (
        <p class="summary">
          {t('project.open', { n: openCount })}
          {openCount > 0 && detail.data ? (gaps ? t('project.unplanned', { n: gaps }) : t('project.allPlanned')) : ''}
        </p>
      )}
      <EventsStrip events={detail.data?.upcoming_events ?? []} label={event => `${dayLabel(event.all_day ? event.start_date! : ymd(new Date(event.start!)), day)} ${eventTime(event)}`} />
      {d && !d.context.heading_id && <DraftRow />}
      {empty && !d ? <Blank title={t('project.empty')} tip={t('project.emptyTip')} /> : <Rows items={loose} opts={{ star: true }} />}
      {heads.map(h => <HeadingSection key={h.id} heading={h} items={open.items!.filter(i => i.heading_id === h.id)} />)}
      <MoreButton rest={open.rest} step={OPEN_BATCH} loading={open.loading} onMore={open.more} />
      <NewHeading projectId={id} />
      {loggedCount > 0 && (
        <button class="logged" aria-expanded={showDone} onClick={() => setShowDone(!showDone)}>
          {t(showDone ? 'project.hideDone' : 'project.showDone', { n: loggedCount })}
        </button>
      )}
      {showDone && loggedCount > 0 && (
        !logged.items ? <Skeleton /> : (
          <>
            <Rows items={logged.items.filter(i => !(i.id in fresh.value))} opts={{ star: true, doneDate: true }} />
            <MoreButton rest={logged.rest} step={30} loading={logged.loading} onMore={logged.more} />
          </>
        )
      )}
    </>
  );
}
