// Focus mode: the timer page or the statistics.
import type { JSX } from 'preact';
import { route } from '../../state/route';
import { Stats } from './Stats';
import { Timer } from './Timer';

export function FocusMain(): JSX.Element {
  return <div id="focus">{route.value.focus === 'stats' ? <Stats /> : <Timer />}</div>;
}
