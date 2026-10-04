// Year view: twelve small months; the shade of orange says how busy each day is.
import type { JSX } from 'preact';
import * as api from '../../api/client';
import { t } from '../../i18n';
import { monthGrid } from '../../lib/dates';
import { heatLevel } from '../../lib/month';
import { useResource } from '../../state/resource';
import { navigate } from '../../state/route';
import { today } from '../../state/store';

export function YearView({ date }: { date: string }): JSX.Element {
  const year = Number(date.slice(0, 4));
  // Only the count per day is fetched, never the events themselves.
  const heat = useResource(`heat:${year}`, () => api.getHeat(year)).data ?? {};
  const letters = t('day.letters').split(',');
  return (
    <div class="yr">
      {Array.from({ length: 12 }, (_, m) => {
        const first = `${year}-${String(m + 1).padStart(2, '0')}-01`;
        return (
          <button key={first} class={`ym ${today.value.slice(0, 7) === first.slice(0, 7) ? 'cur' : ''}`} onClick={() => navigate({ view: 'month', date: first })}>
            <h3>{t('date.m', { m: m + 1 })}</h3>
            <div class="yg">
              {letters.map(x => <em>{x}</em>)}
              {monthGrid(first).map(cell => {
                if (!cell.inMonth) return cell.date < first ? <span /> : null;
                const level = heatLevel(heat[cell.date] ?? 0);
                return <span class={cell.date === today.value ? 'today' : level ? `h${level}` : ''}>{Number(cell.date.slice(8))}</span>;
              })}
            </div>
          </button>
        );
      })}
    </div>
  );
}
