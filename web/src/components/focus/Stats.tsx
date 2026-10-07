// Statistics for the last seven days, after Screen Time: a few figures, a week of stacked bars,
// where the time went, then the sessions day by day.
import type { ComponentChildren, JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { FocusAmount } from '../../api/types';
import { t } from '../../i18n';
import { addDays, hhmm, wall } from '../../lib/dates';
import { chartScale, minutesSpent, sessionsByDay } from '../../lib/focus';
import { dayLabel, span, weekdayShort } from '../../lib/format';
import { serverTime, sessionTitle } from '../../state/focus';
import { useResource } from '../../state/resource';
import { colorOf, projectOf, today } from '../../state/store';
import { Icon } from '../Icons';
import { Blank, ListHead, LoadFailed, Skeleton } from '../items/parts';
import { Tom } from './parts';
import { SessionPop } from './SessionPop';

/** How much of the chart's height the tallest possible bar takes; the rest is room for its number. */
const BAR = 84;
/** Days of records shown before "earlier records" is asked for. */
const RECENT_DAYS = 2;

const projectName = (id: string | null): string => projectOf(id)?.name ?? t('stats.noProject');

function Figure({ label, note, children }: { label: string; note: string; children: ComponentChildren }): JSX.Element {
  return <div><span>{label}</span><b>{children}</b><em>{note}</em></div>;
}

function Line({ at, scale, label, average = false }: { at: number; scale: number; label: string; average?: boolean }): JSX.Element {
  return <div class={average ? 'gl avg' : 'gl'} style={{ bottom: `${(at / scale) * BAR}%` }}><span>{label}</span></div>;
}

function Bar({ tomatoes, projects, scale }: { tomatoes: number; projects: FocusAmount[]; scale: number }): JSX.Element {
  return (
    <div class="cc">
      {tomatoes > 0 && <em>{tomatoes}</em>}
      <div class="cst" style={{ height: `${(tomatoes / scale) * BAR}%` }}>
        {projects.filter(p => p.tomatoes > 0).map(p => (
          <i title={t('stats.segment', { name: projectName(p.project_id), n: p.tomatoes })} style={{ flex: p.tomatoes, background: colorOf(p.project_id) }} />
        ))}
      </div>
    </div>
  );
}

export function Stats(): JSX.Element {
  const day = today.value;
  const from = addDays(day, -6);
  const stats = useResource(`focus-stats:${day}`, api.getFocusStats);
  const log = useResource(`focus-sessions:${from}:${day}`, () => api.listFocusSessions(from, day));
  const [all, setAll] = useState(false);
  /** The record whose popover is open. */
  const [filing, setFiling] = useState<string | null>(null);
  const head = <ListHead icon={<Icon name="chart" />} name={t('focus.stats')} color="var(--blue)" />;
  if (stats.failed || log.failed) return <div class="list">{head}<LoadFailed retry={() => { stats.reload(); log.reload(); }} /></div>;
  if (!stats.data || !log.data) return <div class="list">{head}<Skeleton /></div>;

  const s = stats.data;
  const records = sessionsByDay(log.data, serverTime());
  if (!s.minutes && !s.streak && !records.length) return <div class="list">{head}<Blank title={t('stats.empty')} tip={t('stats.emptyTip')} /></div>;

  const now = s.days[s.days.length - 1]!;
  const scale = chartScale(s.days);
  const average = s.tomatoes / s.days.length;
  const most = Math.max(1, ...s.projects.map(p => p.minutes));
  const shown = all ? records : records.slice(0, RECENT_DAYS);
  // The middle line's label would sit under the average's when the two are close.
  const middle = Math.abs(average - scale / 2) < scale * 0.12 ? '' : String(scale / 2);

  return (
    <div class="list">
      {head}
      <div class="figs">
        <Figure label={t('stats.today')} note={now.minutes ? span(now.minutes) : t('stats.notStarted')}><Tom />{now.tomatoes}</Figure>
        <Figure label={t('stats.week')} note={span(s.minutes)}><Tom />{s.tomatoes}</Figure>
        <Figure label={t('stats.average')} note={span(s.minutes / s.days.length)}>{average.toFixed(1)}<small>{t('stats.unitCount')}</small></Figure>
        <Figure label={t('stats.streak')} note={t(s.streak ? 'stats.streakNote' : 'stats.noStreak')}>{s.streak}<small>{t('stats.unitDays')}</small></Figure>
      </div>
      <div class="chart">
        <Line at={scale} scale={scale} label={String(scale)} />
        <Line at={scale / 2} scale={scale} label={middle} />
        {average > 0 && <Line at={average} scale={scale} label={t('stats.average')} average />}
        {s.days.map(d => <Bar key={d.date} tomatoes={d.tomatoes} projects={d.projects} scale={scale} />)}
      </div>
      <div class="cx">
        {s.days.map(d => <span class={d.date === day ? 'tdy' : ''}>{d.date === day ? t('day.today') : weekdayShort(d.date)}</span>)}
      </div>
      {s.projects.length > 0 && <div class="sec"><span>{t('stats.where')}</span></div>}
      {s.projects.map(p => (
        <div class="srow" style={{ '--c': colorOf(p.project_id) }}>
          <div class="sn"><i class="fdot" /><span title={projectName(p.project_id)}>{projectName(p.project_id)}</span></div>
          <em><Tom />{p.tomatoes} · {span(p.minutes)}</em>
          <div class="sbar"><i style={{ width: `${(p.minutes / most) * 100}%` }} /></div>
        </div>
      ))}
      {shown.map(d => (
        <div key={d.date}>
          <div class="sec">
            <span>{dayLabel(d.date, day)}</span>
            <em><Tom />{d.sessions.filter(x => x.completed).length} · {span(d.sessions.reduce((sum, x) => sum + minutesSpent(x, Infinity), 0))}</em>
          </div>
          {d.sessions.map(x => (
            <div class="ranch" key={x.id}>
              <button class="rrow" style={{ '--c': colorOf(x.project_id) }} aria-expanded={filing === x.id} onClick={() => setFiling(filing === x.id ? null : x.id)}>
                <time>{hhmm(wall(x.start))}–{hhmm(wall(x.end))}</time>
                <i class="fdot" />
                <span title={sessionTitle(x)}>{sessionTitle(x)}</span>
                <em>{x.completed ? <Tom /> : t('stats.unfinished', { dur: span(minutesSpent(x, Infinity)) })}</em>
              </button>
              {filing === x.id && <SessionPop session={x} onClose={() => setFiling(null)} />}
            </div>
          ))}
        </div>
      ))}
      {records.length > shown.length && <button class="morebtn" style="padding-left:2px;margin-top:10px" onClick={() => setAll(true)}>{t('stats.older')}</button>}
    </div>
  );
}
