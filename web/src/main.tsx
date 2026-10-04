import { render } from 'preact';
import { App } from './App';
import { initRoute } from './state/route';
import { boot, startClock } from './state/store';
import { initTheme } from './state/theme';
import './styles/base.css';
import './styles/sidebar.css';
import './styles/items.css';
import './styles/calendar.css';
import './styles/details.css';
import './styles/overlays.css';
import './styles/phone.css';
import './styles/app.css';

initTheme();
initRoute();
render(<App />, document.getElementById('root')!);
void boot();
startClock();
