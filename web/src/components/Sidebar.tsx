// The sidebar navigates within the current mode: items, calendar or focus.
import type { JSX } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import * as api from '../api/client';
import type { Area, Project } from '../api/types';
import { t, type MessageKey } from '../i18n';
import { addMonths, monthGrid, monthStart } from '../lib/dates';
import { cap } from '../lib/format';
import { navigate, route, type Mode } from '../state/route';
import { activeProjects, areas, colorOf, counts, hiddenProjects, today, toggleProjectVisible, write } from '../state/store';
import { FocusSide } from './focus/FocusSide';
import { Icon, Pie, type IconName } from './Icons';
import { InlineText } from './InlineText';
import { Confirm, Popover } from './Popover';
import { ProjectDialog } from './ProjectDialog';

export function setMode(mode: Mode): void {
  navigate({ mode });
  scrollTo(0, 0);
}

function MiniMonth(): JSX.Element {
  const current = route.value.date;
  const [month, setMonth] = useState(monthStart(current));
  // The mini month follows the calendar, and can be paged on its own.
  useEffect(() => setMonth(monthStart(current)), [current]);
  const [y, m] = month.split('-').map(Number);
  return (
    <div class="mini">
      <div class="mini-h">
        <span>{t('date.ym', { y: y!, m: m! })}</span>
        <button aria-label={t('cal.prevMonth')} onClick={() => setMonth(addMonths(month, -1))}><Icon name="left" /></button>
        <button aria-label={t('cal.nextMonth')} onClick={() => setMonth(addMonths(month, 1))}><Icon name="right" /></button>
      </div>
      <div class="mini-g">
        {t('day.letters').split(',').map(x => <em>{x}</em>)}
        {monthGrid(month).map(cell => (
          <button
            key={cell.date}
            class={[cell.inMonth ? '' : 'out', cell.date === today.value ? 'today' : cell.date === current ? 'sel' : ''].join(' ')}
            aria-current={cell.date === current ? 'date' : undefined}
            onClick={() => navigate({ mode: 'cal', view: 'day', date: cell.date })}
          >{Number(cell.date.slice(8))}</button>
        ))}
      </div>
    </div>
  );
}

function CalendarSide(): JSX.Element {
  const hidden = hiddenProjects.value;
  const list = activeProjects.value;
  return (
    <div id="side-cal">
      <MiniMonth />
      <div class="s-label">{t('side.projects')}</div>
      <div class="projects">
        {!list.length ? <p class="hint">{t('side.noProjects')}</p> : list.map(p => {
          const off = hidden.has(p.id);
          return (
            <button key={p.id} class={`proj ${off ? 'off' : ''}`} style={{ '--c': colorOf(p.id) }} aria-pressed={!off} onClick={() => toggleProjectVisible(p.id)}>
              <span class={`pc ${off ? '' : 'on'}`} />
              <span class="pn" title={p.name}>{p.name}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

const NAV: ReadonlyArray<{ id: string; name: MessageKey; icon: IconName; color: string }> = [
  { id: 'inbox', name: 'nav.inbox', icon: 'tray', color: 'var(--blue)' },
  { id: 'today', name: 'nav.today', icon: 'star', color: '#f5b400' },
  { id: 'upcoming', name: 'nav.upcoming', icon: 'cal', color: 'var(--red)' },
  { id: 'matrix', name: 'nav.matrix', icon: 'grid', color: 'var(--orange)' },
  { id: 'all', name: 'nav.all', icon: 'layers', color: 'var(--teal)' },
  { id: 'done', name: 'nav.done', icon: 'book', color: 'var(--green)' },
];

/** Picks a list. On a phone the list sits below the navigation, so it is scrolled to. */
function openList(list: string): void {
  navigate({ mode: 'items', list });
  if (matchMedia('(max-width: 860px)').matches) document.getElementById('list')?.scrollIntoView();
  else scrollTo(0, 0);
}

function ProjectLink({ project }: { project: Project }): JSX.Element {
  const id = `p:${project.id}`;
  return (
    <button class={`nv ${route.value.list === id ? 'on' : ''}`} title={project.name} onClick={() => openList(id)}>
      <Pie done={project.done_count} open={project.open_count} color={colorOf(project.id)} />
      <span class="nl">{project.name}</span>
    </button>
  );
}

function AreaLabel({ area }: { area: Area }): JSX.Element {
  const [open, setOpen] = useState<'menu' | 'rename' | 'delete' | 'new' | null>(null);
  const close = () => setOpen(null);
  return (
    <>
      <div class="area">
        {open === 'rename'
          ? <InlineText value={area.name} label={t('area.name')} autoFocus onSave={name => { void write(api.updateArea(area.id, { name })); }} onDone={close} />
          : <span class="nl" title={area.name}>{area.name}</span>}
        <span class="anch">
          <button class="mbtn" aria-label={t('area.menu')} aria-expanded={open === 'menu'} onClick={() => setOpen(open === 'menu' ? null : 'menu')}><Icon name="more" /></button>
          {open === 'menu' && (
            <Popover onClose={close} class="menu right" label={t('area.menu')}>
              <button onClick={() => setOpen('rename')}>{t('common.rename')}</button>
              <button onClick={() => setOpen('new')}>{t('area.new')}</button>
              <button class="danger" onClick={() => setOpen('delete')}>{t('area.delete')}</button>
            </Popover>
          )}
        </span>
      </div>
      {open === 'new' && (
        <div class="area">
          <InlineText value="" placeholder={t('area.name')} label={t('area.name')} autoFocus onSave={name => { void write(api.createArea(name)); }} onDone={close} />
        </div>
      )}
      {open === 'delete' && (
        <Confirm title={t('area.deleteAsk', { name: area.name })} text={t('area.deleteText')} action={t('common.delete')} onConfirm={() => { void write(api.deleteArea(area.id)); }} onClose={close} />
      )}
    </>
  );
}

function ItemsSide(): JSX.Element {
  const [creating, setCreating] = useState(false);
  const list = activeProjects.value;
  const known = new Set(areas.value.map(a => a.id));
  const count: Record<string, number> = { inbox: counts.value.inbox, today: counts.value.today };
  const selected = route.value.list;
  return (
    <div id="side-tasks">
      <div class="nav">
        {NAV.map(n => (
          <button key={n.id} class={`nv ${selected === n.id ? 'on' : ''}`} style={{ '--c': n.color }} aria-current={selected === n.id ? 'page' : undefined} onClick={() => openList(n.id)}>
            <Icon name={n.icon} />
            <span class="nl">{t(n.name)}</span>
            {count[n.id] ? <span class="n">{cap(count[n.id]!)}</span> : null}
          </button>
        ))}
        {list.some(p => !p.area_id || !known.has(p.area_id)) && <div class="gap" />}
        {list.filter(p => !p.area_id || !known.has(p.area_id)).map(p => <ProjectLink key={p.id} project={p} />)}
        {areas.value.map(area => (
          <div key={area.id} class="nav">
            <AreaLabel area={area} />
            {list.filter(p => p.area_id === area.id).map(p => <ProjectLink key={p.id} project={p} />)}
          </div>
        ))}
        <button class="nv add" onClick={() => setCreating(true)}><Icon name="plus" />{t('project.new')}</button>
      </div>
      {creating && <ProjectDialog onClose={() => setCreating(false)} />}
    </div>
  );
}

export function Sidebar(): JSX.Element {
  const mode = route.value.mode;
  return (
    <aside class="sidebar glass">
      <div class="seg mode" role="group" aria-label={t('mode.label')}>
        <button class={mode === 'items' ? 'on' : ''} aria-pressed={mode === 'items'} onClick={() => setMode('items')}>{t('mode.items')}</button>
        <button class={mode === 'cal' ? 'on' : ''} aria-pressed={mode === 'cal'} onClick={() => setMode('cal')}>{t('mode.cal')}</button>
        <button class={mode === 'focus' ? 'on' : ''} aria-pressed={mode === 'focus'} onClick={() => setMode('focus')}>{t('mode.focus')}</button>
      </div>
      <CalendarSide />
      <ItemsSide />
      {mode === 'focus' && <FocusSide />}
    </aside>
  );
}
