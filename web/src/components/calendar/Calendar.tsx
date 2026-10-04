// The calendar in its current view, with the events of the visible range.
import type { JSX } from 'preact';
import { useEffect } from 'preact/hooks';
import type { CalEvent } from '../../api/types';
import { viewRange } from '../../lib/calendar';
import { weekDays } from '../../lib/dates';
import { closeCalPop } from '../../state/calendar';
import { useEvents } from '../../state/events';
import { route } from '../../state/route';
import { hiddenProjects } from '../../state/store';
import { MonthView } from './MonthView';
import { TimeGrid } from './TimeGrid';
import { YearView } from './YearView';

const NONE: readonly CalEvent[] = [];

function EventViews({ view, date }: { view: 'day' | 'week' | 'month'; date: string }): JSX.Element {
  const { range, neighbours } = viewRange(view, date);
  const hidden = hiddenProjects.value;
  // The grid is drawn at once; events appear in it when they arrive.
  const events = (useEvents(range, neighbours) ?? NONE).filter(e => !e.project_id || !hidden.has(e.project_id));
  if (view === 'month') return <MonthView date={date} events={events} />;
  return <TimeGrid key={view} days={view === 'week' ? weekDays(date) : [date]} events={events} />;
}

export function Calendar(): JSX.Element {
  const { view, date } = route.value;
  useEffect(() => closeCalPop, [view, date]);
  return (
    <div id="cal">
      {view === 'year' ? <YearView date={date} /> : <EventViews key={view === 'month' ? 'month' : 'grid'} view={view} date={date} />}
    </div>
  );
}
