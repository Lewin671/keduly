// @vitest-environment happy-dom
// Mounts the whole app against a fake server, to catch what type-checking cannot: a screen that
// throws while rendering, or that asks the API for the wrong thing.
import { render } from 'preact';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Activity, Actor, Bootstrap, CalEvent, Focus, FocusSession, FocusStats, Item, Suggestion } from './api/types';

const me: Actor = { kind: 'user', name: 'Me' };
const agent: Actor = { kind: 'agent', name: 'Claude Code' };
const DAY = '2026-10-13';
const at = (time: string, day = DAY) => new Date(`${day}T${time}:00`).toISOString().replace('.000Z', 'Z');

const item = (id: string, title: string, fields: Partial<Item> = {}): Item => ({
  id, project_id: null, heading_id: null, title, notes: '', estimate_minutes: null, planned_date: null, evening: false,
  due_date: null, due_time: null, important: false, status: 'open', completed_at: null, position: 0, block: null,
  suggestion: null, focus: { tomatoes: 0, minutes: 0 }, created_by: me, created_at: at('08:00'), updated_at: at('08:00'), ...fields,
});
const event = (id: string, title: string, fields: Partial<CalEvent> = {}): CalEvent => ({
  id, project_id: 'p1', item_id: null, title, notes: '', location: '', all_day: false, start: at('10:00'), end: at('11:30'),
  start_date: null, end_date: null, rrule: null, recurring: false, instance: null, status: 'confirmed', suggestion_id: null,
  item_done: null, readonly: false, created_by: me, updated_at: at('08:00'), ...fields,
});

const bootstrap: Bootstrap = {
  user: { id: 'u1', email: 'me@example.com', name: 'Me', timezone: 'America/New_York', timezone_auto: false, work_start: '09:00', work_end: '18:00', focus_minutes: 25, rest_minutes: 5, long_rest_minutes: 15, round_size: 4, created_at: at('08:00') },
  areas: [{ id: 'a1', name: '工作', position: 0 }],
  projects: [
    { id: 'p1', area_id: 'a1', name: 'Keduly 开发', color: 'blue', notes: '给 AI 用的日历', position: 0, archived: false, open_count: 3, done_count: 2, created_at: at('08:00'), updated_at: at('08:00') },
    { id: 'p2', area_id: null, name: '生活', color: 'green', notes: '', position: 1, archived: false, open_count: 0, done_count: 0, created_at: at('08:00'), updated_at: at('08:00') },
  ],
  headings: [{ id: 'h1', project_id: 'p1', name: '同步', position: 0 }],
  counts: { inbox: 120, today: 3, pending: 2 },
  revision: 7,
};

const suggestion: Suggestion = {
  id: 's1', status: 'pending', kind: 'schedule_item', actor: agent, reason: '今天 18:00 截止，这是下午第一个 1 小时空档。', title: '写周报',
  item_id: 'i3', event_id: null, start: at('14:00'), end: at('15:00'), item: null, event: null, created_at: at('08:00'), decided_at: null,
};
const deletion: Suggestion = { ...suggestion, id: 's2', kind: 'delete_event', title: '1:1 与王敏', reason: '王敏本周请假', item_id: null, event_id: 'e9', start: null, end: null };

const caldav = item('i1', '实现 CalDAV 同步', { project_id: 'p1', heading_id: 'h1', estimate_minutes: 90, planned_date: DAY, important: true, focus: { tomatoes: 2, minutes: 50 }, block: { event_id: 'b1', start: at('15:30'), end: at('17:00') } });
const reply = item('i2', '回复 PR 评论', { project_id: 'p1', estimate_minutes: 30, planned_date: DAY, due_date: DAY });
const weekly = item('i3', '写周报', { planned_date: DAY, estimate_minutes: 60, suggestion: { id: 's1', start: at('14:00'), end: at('15:00'), reason: suggestion.reason, actor: agent } });
const checkup = item('i4', '预约体检', { planned_date: DAY, evening: true });
const tax = item('i5', '提交个税专项附加扣除', { due_date: '2026-10-09' });
const finished = item('i6', '整理需求文档', { project_id: 'p1', status: 'done', completed_at: at('17:40', '2026-10-12') });

const events: CalEvent[] = [
  event('e1', '设计评审'),
  event('e2', '候选人面试', { start: at('10:15'), end: at('11:00'), project_id: 'p2' }),
  event('b1', '实现 CalDAV 同步', { item_id: 'i1', start: at('15:30'), end: at('17:00'), item_done: false }),
  event('t1', '写周报', { item_id: 'i3', project_id: null, start: at('14:00'), end: at('15:00'), status: 'tentative', suggestion_id: 's1' }),
  event('e3', '国庆假期', { all_day: true, start: null, end: null, start_date: '2026-10-12', end_date: '2026-10-14' }),
  event('e4', '站会', { start: at('09:00'), end: at('09:15'), recurring: true, readonly: true, instance: at('09:00'), rrule: 'FREQ=DAILY' }),
];

const activity: Activity = { id: 'act1', actor: agent, action: 'item.create', summary: '新建事项「调研 FullCalendar 授权」', reason: null, undoable: true, undone: false, created_at: at('07:58') };

const session = (id: string, start: string, fields: Partial<FocusSession> = {}): FocusSession => ({
  id, kind: 'work', item_id: 'i2', project_id: 'p1', title: '回复 PR 评论', start,
  end: new Date(Date.parse(start) + 25 * 60_000).toISOString().replace('.000Z', 'Z'), planned_minutes: 25, completed: true, created_by: me, ...fields,
});
/** The timer with one tomato earned today and nothing running. */
const idle: Focus = { state: 'idle', session: null, tomatoes_today: 1, minutes_today: 25, round_size: 4, round_done: 1, rest_minutes: 5, now: at('10:40') };
/** A tomato started on "回复 PR 评论" at 10:40. */
const working: Focus = { ...idle, state: 'work', session: session('f2', at('10:40'), { completed: false }) };
const stats: FocusStats = {
  days: ['07', '08', '09', '10', '11', '12', '13'].map((d, k) => ({
    date: `2026-10-${d}`, tomatoes: [2, 4, 4, 3, 0, 6, 1][k]!, minutes: [50, 100, 100, 75, 0, 162, 25][k]!,
    projects: k === 4 ? [] : [{ project_id: k % 2 ? 'p1' : null, tomatoes: [2, 4, 4, 3, 0, 6, 1][k]!, minutes: [50, 100, 100, 75, 0, 162, 25][k]! }],
  })),
  projects: [{ project_id: 'p1', tomatoes: 11, minutes: 287 }, { project_id: null, tomatoes: 9, minutes: 225 }],
  tomatoes: 20, minutes: 512, streak: 2,
};
const log: FocusSession[] = [
  session('f0', at('10:00', '2026-10-12'), { item_id: 'i6', title: '整理需求文档' }),
  session('f9', at('16:00', '2026-10-12'), { item_id: null, project_id: null, title: '', end: at('16:12', '2026-10-12'), completed: false }),
  session('f1', at('09:32')),
];

const quadrant = (items: Item[]) => ({ total: items.length, unplanned: items.filter(i => !i.block && !i.suggestion).length, items });

/** What the fake server answers, by method and path. */
function routes(): Record<string, unknown> {
  return {
    'GET /config': { registration: 'open', version: 'test' },
    'GET /bootstrap': bootstrap,
    'GET /counts': { counts: bootstrap.counts, revision: bootstrap.revision },
    'GET /suggestions': { suggestions: [suggestion, deletion] },
    'GET /events': { events },
    'GET /calendar/heat': { days: { [DAY]: 7, '2026-10-14': 3, '2026-03-02': 1 } },
    'GET /today': { date: DAY, items: [reply, caldav, weekly, checkup], overdue: [tax], events: [events[4], events[0]], free_minutes: 210, unplanned_minutes: 30 },
    'GET /upcoming': { days: [{ date: '2026-10-14', events: [event('e5', '客户演示', { start: at('10:00', '2026-10-14'), end: at('11:30', '2026-10-14') })], items: [item('i7', '准备演示环境', { due_date: '2026-10-14' })] }] },
    'GET /overview': { projects: [{ project_id: 'p1', total: 6, items: [caldav, reply, item('o1', '甲'), item('o2', '乙'), item('o3', '丙')] }] },
    'GET /matrix': { quadrants: { do: quadrant([tax]), plan: quadrant([caldav]), quick: quadrant([reply]), later: quadrant([]) } },
    'GET /items': { items: [caldav, reply], next_cursor: null, total: 2 },
    'GET /projects/p1': { project: bootstrap.projects[0], headings: bootstrap.headings, upcoming_events: [events[0]], unplanned_count: 1, focus_week_minutes: 187 },
    'GET /focus': { focus: idle },
    'GET /focus/stats': stats,
    'GET /focus/sessions': { sessions: log },
    'GET /activity': { activities: [activity], next_cursor: 'older' },
    'GET /tokens': { tokens: [{ id: 'k1', name: 'Claude Code · MacBook', kind: 'agent', scope: 'write', confirm_delete: true, last_used_at: at('08:00'), created_at: at('08:00') }] },
    'GET /free': { slots: [{ start: at('13:00'), end: at('14:00') }] },
  };
}

interface Call {
  method: string;
  path: string;
  query: URLSearchParams;
  body: unknown;
}

let calls: Call[] = [];
let table: Record<string, unknown> = {};

function fakeFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const url = new URL(input, 'http://app.test');
  const method = init.method ?? 'GET';
  const path = url.pathname.replace('/api/v1', '');
  calls.push({ method, path, query: url.searchParams, body: typeof init.body === 'string' ? JSON.parse(init.body) : undefined });
  const answer = table[`${method} ${path}`];
  if (answer === undefined) return Promise.resolve(new Response(JSON.stringify({ error: { code: 'not_found', message: `${method} ${path}` } }), { status: 404 }));
  if (answer === 401) return Promise.resolve(new Response(JSON.stringify({ error: { code: 'unauthenticated', message: 'sign in' } }), { status: 401 }));
  if (answer === 204) return Promise.resolve(new Response(null, { status: 204 }));
  return Promise.resolve(new Response(JSON.stringify(answer), { status: 200 }));
}

const settle = async () => { for (let k = 0; k < 12; k++) await new Promise(resolve => setTimeout(resolve, 0)); };
const text = () => document.body.textContent ?? '';
const find = (selector: string, label: string): HTMLElement => {
  const el = [...document.querySelectorAll<HTMLElement>(selector)].find(e => (e.textContent ?? '').includes(label) || e.getAttribute('aria-label') === label);
  if (!el) throw new Error(`no ${selector} with "${label}"`);
  return el;
};
/** Types into a field and lets the app re-render before the next step. */
async function type(input: HTMLInputElement | HTMLTextAreaElement, value: string): Promise<void> {
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  await settle();
}
const callsTo = (method: string, path: string) => calls.filter(c => c.method === method && c.path === path);

type Modules = {
  App: typeof import('./App').App;
  store: typeof import('./state/store');
  route: typeof import('./state/route');
  focus: typeof import('./state/focus');
  link: typeof import('./state/link');
};
let mod: Modules;

async function open(hash: string): Promise<void> {
  mod.route.navigate(mod.route.parseRoute(hash, mod.route.route.value));
  await mod.store.enter();
  await settle();
}

beforeAll(async () => {
  vi.useFakeTimers({ toFake: ['Date'], now: new Date(`${DAY}T10:40:00`) });
  vi.stubGlobal('fetch', fakeFetch);
  // The DOM shim scrolls nothing and has no element scrolling.
  Element.prototype.scrollIntoView = () => {};
  window.scrollTo = () => {};
  const [{ App }, store, route, focus, link] = await Promise.all([import('./App'), import('./state/store'), import('./state/route'), import('./state/focus'), import('./state/link')]);
  mod = { App, store, route, focus, link };
});

beforeEach(() => {
  calls = [];
  table = routes();
  document.body.innerHTML = '<div id="root"></div>';
  mod.store.now.value = new Date();
  render(<mod.App />, document.getElementById('root')!);
});

afterEach(() => {
  render(null, document.getElementById('root')!);
  mod.link.link.value = null;
});

describe('session', () => {
  it('shows sign-in without a session, and sign-up only when registration is open', async () => {
    table['GET /bootstrap'] = 401;
    await mod.store.boot();
    await settle();
    expect(document.querySelector('.auth-card h1')?.textContent).toBe('Keduly');
    expect(text()).toContain('还没有账号？注册');

    mod.store.config.value = { registration: 'closed', version: 'test' };
    await settle();
    expect(text()).not.toContain('注册');
  });

  it('registers with the browser time zone, then loads the app', async () => {
    table['GET /bootstrap'] = 401;
    await mod.store.boot();
    await settle();
    find('button', '还没有账号？注册').click();
    await settle();
    const [name, email, password] = [...document.querySelectorAll<HTMLInputElement>('.auth-card input')];
    await type(name!, 'Me');
    await type(email!, 'me@example.com');
    await type(password!, 'correct horse');
    table['POST /auth/register'] = { user: bootstrap.user };
    table['GET /bootstrap'] = bootstrap;
    document.querySelector<HTMLFormElement>('.auth-card')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await settle();
    expect(callsTo('POST', '/auth/register')[0]!.body).toEqual({ email: 'me@example.com', password: 'correct horse', name: 'Me', timezone: 'America/New_York' });
    expect(document.querySelector('.app')).not.toBeNull();
  });
});

describe('signing in on another device', () => {
  const submit = () => document.querySelector<HTMLFormElement>('.link-card')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

  it('reads a scanned link and nothing else from the hash', () => {
    const { parseLink } = mod.link;
    expect(parseLink('#approve=abcdefgh234567ab')).toEqual({ kind: 'approve', id: 'abcdefgh234567ab' });
    expect(parseLink('#signin=Zm9v_bar-1')).toEqual({ kind: 'signin', code: 'Zm9v_bar-1' });
    for (const hash of ['', '#/items/today', '#signin=', '#signin=a b', '#approve=a/b', '#other=abc', 'signin=abc']) expect(parseLink(hash)).toBeNull();
  });

  it('shows a QR code and a pin on the sign-in screen, and enters once another device allows it', async () => {
    table['GET /bootstrap'] = 401;
    await mod.store.boot();
    await settle();
    table['POST /auth/requests'] = { request: { id: 'r1', device: '', expires_at: at('10:42') }, pin: '4821', secret: 'only-here' };
    table['POST /auth/requests/r1/claim'] = { status: 'pending' };
    find('button', '扫码登录').click();
    await settle();
    expect(document.querySelector('svg.qr path')?.getAttribute('d')).toMatch(/^M\d+ \d+h1v1h-1z/);
    expect(document.querySelector('.pin-show')!.textContent).toBe('4821');
    // The QR code must not carry the secret or the pin.
    expect(document.querySelector('.qr-card')!.innerHTML).not.toContain('only-here');

    table['POST /auth/requests/r1/claim'] = { status: 'approved', user: bootstrap.user };
    table['GET /bootstrap'] = bootstrap;
    await new Promise(resolve => setTimeout(resolve, 2200));
    await settle();
    expect(callsTo('POST', '/auth/requests/r1/claim')[0]!.body).toEqual({ secret: 'only-here' });
    expect(document.querySelector('.app')).not.toBeNull();
    expect(document.querySelector('.auth')).toBeNull();
  });

  it('offers a new QR code when the request is gone', async () => {
    table['GET /bootstrap'] = 401;
    await mod.store.boot();
    await settle();
    table['POST /auth/requests'] = { request: { id: 'r2', device: '', expires_at: at('10:42') }, pin: '0007', secret: 's' };
    find('button', '扫码登录').click();
    await new Promise(resolve => setTimeout(resolve, 2200));
    await settle();
    expect(text()).toContain('二维码已过期');
    expect(document.querySelector('svg.qr')).toBeNull();
    find('button', '重新生成').click();
    await settle();
    expect(callsTo('POST', '/auth/requests')).toHaveLength(2);
    find('button', '用密码登录').click();
    await settle();
    expect(document.querySelector('.auth-card input[type=password]')).not.toBeNull();
  });

  it('asks a signed-in device for the pin before letting the other one in', async () => {
    table['GET /auth/requests/r1'] = { request: { id: 'r1', device: 'Chrome · Mac', expires_at: at('10:42') } };
    mod.link.link.value = { kind: 'approve', id: 'r1' };
    await open('#/items/today');
    expect(text()).toContain('一台设备（Chrome · Mac）想登录你的账号 me@example.com');
    const allow = find('.link-card button', '允许登录') as HTMLButtonElement;
    expect(allow.disabled).toBe(true);
    await type(document.querySelector<HTMLInputElement>('.fld.pin')!, '48a21');
    expect(document.querySelector<HTMLInputElement>('.fld.pin')!.value).toBe('4821');
    table['POST /auth/requests/r1/approve'] = 204;
    submit();
    await settle();
    expect(callsTo('POST', '/auth/requests/r1/approve')[0]!.body).toEqual({ pin: '4821' });
    expect(text()).toContain('已允许');
    find('.link-card button', '完成').click();
    await settle();
    expect(document.querySelector('.auth')).toBeNull();
    expect(document.querySelector('.app')).not.toBeNull();
  });

  it('refuses a request, and says so when a scanned request no longer exists', async () => {
    table['GET /auth/requests/r1'] = { request: { id: 'r1', device: '', expires_at: at('10:42') } };
    table['DELETE /auth/requests/r1'] = 204;
    mod.link.link.value = { kind: 'approve', id: 'r1' };
    await open('#/items/today');
    expect(text()).toContain('一台设备想登录你的账号');
    find('.link-card button', '拒绝').click();
    await settle();
    expect(callsTo('DELETE', '/auth/requests/r1')).toHaveLength(1);
    expect(document.querySelector('.auth')).toBeNull();

    mod.link.link.value = { kind: 'approve', id: 'r9' };
    await settle();
    expect(text()).toContain('二维码已失效');
    expect(document.querySelector('.fld.pin')).toBeNull();
  });

  it('asks for the password first when the scanning device is not signed in', async () => {
    table['GET /bootstrap'] = 401;
    mod.link.link.value = { kind: 'approve', id: 'r1' };
    await mod.store.boot();
    await settle();
    expect(text()).toContain('先登录，再允许另一台设备登录');
    expect(text()).not.toContain('扫码登录');
    expect(callsTo('GET', '/auth/requests/r1')).toHaveLength(0);
  });

  it('signs in with a scanned code only after the user confirms', async () => {
    table['GET /bootstrap'] = 401;
    table['POST /auth/codes/check'] = { name: 'Me', email: 'me@example.com' };
    mod.link.link.value = { kind: 'signin', code: 'one-time' };
    await mod.store.boot();
    await settle();
    expect(text()).toContain('将登录 Me 的账号 me@example.com');
    expect(callsTo('POST', '/auth/codes/redeem')).toHaveLength(0);
    table['POST /auth/codes/redeem'] = { user: bootstrap.user };
    table['GET /bootstrap'] = bootstrap;
    submit();
    await settle();
    expect(callsTo('POST', '/auth/codes/redeem')[0]!.body).toEqual({ code: 'one-time' });
    expect(document.querySelector('.auth')).toBeNull();
    expect(document.querySelector('.app')).not.toBeNull();
  });

  it('does not use a code up on a device that already has the account, and reports a dead code', async () => {
    table['POST /auth/codes/check'] = { name: 'Me', email: 'me@example.com' };
    mod.link.link.value = { kind: 'signin', code: 'one-time' };
    await open('#/items/today');
    expect(text()).toContain('这台设备已经登录了这个账号');

    delete table['POST /auth/codes/check'];
    mod.link.link.value = { kind: 'signin', code: 'used' };
    await settle();
    expect(text()).toContain('二维码已失效');
    expect(callsTo('POST', '/auth/codes/redeem')).toHaveLength(0);
  });

  it('shows a sign-in code in settings and withdraws it when closed', async () => {
    table['POST /auth/codes'] = { code: 'one-time', expires_at: at('10:42') };
    table['DELETE /auth/codes'] = 204;
    await open('#/items/today');
    find('.rbtn', '设置').click();
    await settle();
    find('.g-row', '在其他设备登录…').click();
    await settle();
    expect(document.querySelector('.other-device svg.qr')).not.toBeNull();
    expect(text()).toContain('它相当于临时密码');
    expect(callsTo('DELETE', '/auth/codes')).toHaveLength(0);
    find('.other-device button', '完成').click();
    await settle();
    expect(callsTo('DELETE', '/auth/codes')).toHaveLength(1);
    expect(document.querySelector('.other-device')).toBeNull();
  });
});

describe('calendar', () => {
  it('draws the day view: title, all-day row, lanes, time block, tentative entry, now line', async () => {
    await open(`#/cal/day/${DAY}`);
    expect(document.body.dataset.mode).toBe('cal');
    expect(document.getElementById('title')!.textContent).toBe('10月13日星期二');
    expect(document.querySelector('.allday .ad')!.textContent).toBe('国庆假期');
    const blocks = [...document.querySelectorAll<HTMLElement>('.blk:not(.ses)')];
    expect(blocks.map(b => b.querySelector('.bt')!.textContent)).toEqual(['站会', '设计评审', '候选人面试', '写周报', '实现 CalDAV 同步']);
    // The two overlapping events share the column.
    expect(blocks[1]!.style.width).toBe('calc(50% - 4px)');
    expect(blocks[2]!.style.left).toBe('calc(50% + 2px)');
    expect(blocks[3]!.className).toContain('tent');
    expect(blocks[3]!.querySelector('.bm')!.textContent).toBe('14:00–15:00 · 待定');
    expect(blocks[4]!.className).toContain('task');
    expect(blocks[4]!.querySelector('.chk')).not.toBeNull();
    expect(blocks[0]!.className).toContain('short');
    expect(document.querySelector('.nowcap')!.textContent).toBe('10:40');
    expect(parseFloat(document.querySelector<HTMLElement>('.now')!.style.top)).toBeCloseTo((10 + 40 / 60) * 58, 3);
    // The whole week is requested, as UTC bounds of local days.
    const q = callsTo('GET', '/events')[0]!.query;
    expect([q.get('from'), q.get('to')]).toEqual(['2026-10-12T04:00:00Z', '2026-10-19T04:00:00Z']);
  });

  it('prefetches the weeks before and after', async () => {
    await open(`#/cal/day/${DAY}`);
    expect(callsTo('GET', '/events').map(c => c.query.get('from')).sort()).toEqual(['2026-10-05T04:00:00Z', '2026-10-12T04:00:00Z', '2026-10-19T04:00:00Z']);
  });

  it('hides the events of a project switched off in the sidebar', async () => {
    await open(`#/cal/day/${DAY}`);
    find('.proj', '生活').click();
    await settle();
    expect([...document.querySelectorAll('.blk .bt')].map(b => b.textContent)).not.toContain('候选人面试');
    find('.proj', '生活').click();
    await settle();
    expect([...document.querySelectorAll('.blk .bt')].map(b => b.textContent)).toContain('候选人面试');
  });

  it('opens the suggestion popover and accepts with an undo', async () => {
    await open(`#/cal/day/${DAY}`);
    find('.blk', '写周报').click();
    await settle();
    const pop = document.querySelector('.pop')!;
    expect(pop.textContent).toContain('排到今天 14:00');
    expect(pop.textContent).toContain('Claude Code 的建议');
    expect(pop.textContent).toContain('这是下午第一个 1 小时空档');
    table['POST /suggestions/s1/accept'] = { suggestion: { ...suggestion, status: 'accepted' }, activity };
    table['POST /activity/act1/undo'] = { activity: { ...activity, undone: true } };
    find('.pop .pbtn', '接受').click();
    await settle();
    expect(document.querySelector('.hud')!.textContent).toContain('已接受「写周报」');
    find('.hud button', '撤销').click();
    await settle();
    expect(callsTo('POST', '/activity/act1/undo')).toHaveLength(1);
  });

  it('shows a recurring instance read-only and an ordinary event in the editor', async () => {
    await open(`#/cal/day/${DAY}`);
    find('.blk', '站会').click();
    await settle();
    expect(document.querySelector('.pop')!.textContent).toContain('在系统日历里修改重复日程');
    expect(document.querySelector('.pop input')).toBeNull();
    find('.blk', '设计评审').click();
    await settle();
    expect(document.querySelector<HTMLInputElement>('.pop input.title')!.value).toBe('设计评审');
    expect(text()).toContain('删除');
  });

  it('creates an event from the toolbar with UTC times', async () => {
    await open(`#/cal/day/${DAY}`);
    find('.rbtn', '新建日程').click();
    await settle();
    await type(document.querySelector<HTMLInputElement>('.pop input.title')!, '午饭');
    table['POST /events'] = { event: event('e7', '午饭') };
    document.querySelector<HTMLFormElement>('.pop form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await settle();
    // 10:40 local: the next full hour is 11:00 EDT, 15:00 UTC.
    expect(callsTo('POST', '/events')[0]!.body).toEqual({ title: '午饭', project_id: null, notes: '', all_day: false, start: '2026-10-13T15:00:00Z', end: '2026-10-13T16:00:00Z' });
  });

  it('checks off a time block at once and tells the server', async () => {
    await open(`#/cal/day/${DAY}`);
    table['PATCH /items/i1'] = { item: { ...caldav, status: 'done' } };
    find('.blk', '实现 CalDAV 同步').querySelector<HTMLElement>('.chk')!.click();
    await Promise.resolve();
    expect(find('.blk', '实现 CalDAV 同步').className).toContain('done');
    await settle();
    expect(callsTo('PATCH', '/items/i1')[0]!.body).toEqual({ status: 'done' });
  });

  it('resizes a time block by dragging its bottom edge, snapping to a quarter of an hour', async () => {
    await open(`#/cal/day/${DAY}`);
    table['POST /items/i1/schedule'] = { item: caldav };
    const handle = find('.blk', '实现 CalDAV 同步').querySelector<HTMLElement>('.rz')!;
    const pointer = (type: string, y: number) => new PointerEvent(type, { bubbles: true, pointerId: 1, button: 0, pointerType: 'mouse', clientX: 10, clientY: y });
    handle.dispatchEvent(pointer('pointerdown', 100));
    // 58px is one hour in the day view; 50px is 52 minutes, which snaps to 45.
    window.dispatchEvent(pointer('pointermove', 150));
    await settle();
    expect(find('.blk', '实现 CalDAV 同步').querySelector('.bm')!.textContent).toBe('15:30–17:45');
    window.dispatchEvent(pointer('pointerup', 150));
    await settle();
    expect(callsTo('POST', '/items/i1/schedule')[0]!.body).toEqual({ start: '2026-10-13T19:30:00Z', end: '2026-10-13T21:45:00Z' });
  });

  it('moves an event by dragging it, and puts it back when the server refuses', async () => {
    await open(`#/cal/day/${DAY}`);
    const block = () => find('.blk', '设计评审');
    const pointer = (type: string, y: number) => new PointerEvent(type, { bubbles: true, pointerId: 1, button: 0, pointerType: 'mouse', clientX: 10, clientY: y });
    block().dispatchEvent(pointer('pointerdown', 100));
    window.dispatchEvent(pointer('pointermove', 100 - 58));
    window.dispatchEvent(pointer('pointerup', 100 - 58));
    await settle();
    // No PATCH route is defined: the server answered 404.
    expect(callsTo('PATCH', '/events/e1')[0]!.body).toEqual({ start: '2026-10-13T13:00:00Z', end: '2026-10-13T14:30:00Z' });
    expect(block().querySelector('.bm')!.textContent).toBe('10:00–11:30');
    expect(document.querySelector('.toast')).not.toBeNull();
  });

  it('does not let a recurring instance or a tentative entry be dragged', async () => {
    await open(`#/cal/day/${DAY}`);
    expect(find('.blk', '站会').querySelector('.rz')).toBeNull();
    expect(find('.blk', '写周报').querySelector('.rz')).toBeNull();
  });

  it('starts a new event where an empty slot is clicked', async () => {
    await open(`#/cal/day/${DAY}`);
    const col = document.querySelector<HTMLElement>('.col')!;
    // A click right after a popover was dismissed is ignored; an earlier test may have just closed one.
    await new Promise(resolve => setTimeout(resolve, 400));
    // The shim lays nothing out, so the column starts at y = 0: 58px per hour puts 493px at 08:30.
    col.dispatchEvent(new MouseEvent('click', { bubbles: true, clientY: 493 }));
    await settle();
    const times = [...document.querySelectorAll<HTMLInputElement>('.pop input[type="time"]')].map(i => i.value);
    expect(times).toEqual(['08:30', '09:30']);
    // Dismissing it must not start another one from the same click.
    document.dispatchEvent(new Event('pointerdown', { bubbles: true }));
    col.dispatchEvent(new MouseEvent('click', { bubbles: true, clientY: 100 }));
    await settle();
    expect(document.querySelector('.pop')).toBeNull();
  });

  it('draws the week view with its ISO week and a spanning all-day bar', async () => {
    await open(`#/cal/week/${DAY}`);
    expect(document.getElementById('title')!.textContent).toBe('2026年10月第 42 周');
    expect(document.querySelectorAll('.wk .col')).toHaveLength(7);
    expect(document.querySelector('.wk-head.today')!.textContent).toBe('周二13');
    expect(document.querySelector<HTMLElement>('.wk-ad .ad')!.style.gridColumn).toBe('1 / 4');
  });

  it('draws the month view with the four-row cap', async () => {
    table['GET /events'] = { events: [...events, event('e8', '健身', { start: at('18:30'), end: at('19:30') })] };
    await open(`#/cal/month/${DAY}`);
    expect(document.getElementById('title')!.textContent).toBe('2026年10月');
    expect(document.querySelectorAll('.mcell')).toHaveLength(35);
    const cell = document.querySelector('.mcell.today')!;
    expect([...cell.querySelectorAll('.me')].map(e => e.querySelector('.mt')?.textContent ?? e.textContent)).toEqual(['国庆假期', '站会', '写周报']);
    expect(cell.querySelector('.more')!.textContent).toBe('还有 4 项');
    expect(cell.querySelector('.me.tent .mx')!.textContent).toBe('待定');
  });

  it('draws the year view from the heat map and opens a month', async () => {
    await open(`#/cal/year/${DAY}`);
    expect(document.getElementById('title')!.textContent).toBe('2026年');
    expect(callsTo('GET', '/calendar/heat')[0]!.query.get('year')).toBe('2026');
    expect(document.querySelectorAll('.ym')).toHaveLength(12);
    expect(document.querySelector('.ym.cur h3')!.textContent).toBe('10月');
    expect(document.querySelector('.yg .today')!.textContent).toBe('13');
    expect(document.querySelectorAll('.yg .h2')).toHaveLength(1);
    find('.ym', '3月').click();
    await settle();
    expect(location.hash).toBe('#/cal/month/2026-03-01');
  });

  it('pages with the toolbar and returns to today', async () => {
    await open(`#/cal/week/${DAY}`);
    find('.nav-cap button', '向后').click();
    await settle();
    expect(location.hash).toBe('#/cal/week/2026-10-20');
    find('.nav-cap button', '今天').click();
    await settle();
    expect(location.hash).toBe(`#/cal/week/${DAY}`);
  });
});

describe('items', () => {
  it('lists today with its summary, overdue and evening sections', async () => {
    await open('#/items/today');
    expect(document.body.dataset.mode).toBe('tasks');
    expect(document.querySelector('.summary')!.textContent).toBe('18:00 前还有 3.5 小时空闲，1 件事项未安排（30 分钟）');
    expect([...document.querySelectorAll('.sec')].map(s => s.textContent)).toEqual(['逾期1', '今天3', '今晚']);
    // Important items lead the day.
    expect([...document.querySelectorAll('.todo .tt')].map(e => e.textContent)).toEqual(['提交个税专项附加扣除', '!实现 CalDAV 同步', '回复 PR 评论', '写周报', '预约体检']);
    expect(text()).toContain('逾期 4 天');
    expect(text()).toContain('待定 今天 14:00');
    expect(find('.nv', '收件箱').querySelector('.n')!.textContent).toBe('99+');
    expect(document.querySelector('.badge')!.textContent).toBe('2');
  });

  it('checks an item off at once and rolls back when the server refuses', async () => {
    await open('#/items/today');
    const row = () => find('.todo', '回复 PR 评论');
    row().querySelector<HTMLElement>('.chk')!.click();
    await Promise.resolve();
    expect(row().className).toContain('done');
    await settle();
    // No PATCH route is defined, so the server answered 404.
    expect(row().className).not.toContain('done');
    expect(document.querySelector('.toast')!.textContent).toBe('找不到这项内容，可能已被删除');
  });

  it('opens an item as a card with its suggestion and chips', async () => {
    await open('#/items/today');
    find('.todo .tm', '写周报').click();
    await settle();
    const card = document.querySelector('.todo.open')!;
    expect(card.querySelector<HTMLTextAreaElement>('textarea.tt')!.value).toBe('写周报');
    expect(card.querySelector('.sg')!.textContent).toContain('Claude Code 建议：排到今天 14:00。今天 18:00 截止');
    expect([...card.querySelectorAll('.cm .ck')].map(c => c.textContent)).toEqual(['待定 今天 14:00', '1 小时', '开始专注', '截止日期', '!标为重要', '收件箱', '']);
  });

  it('schedules an item from its card', async () => {
    await open('#/items/today');
    find('.todo .tm', '回复 PR 评论').click();
    await settle();
    find('.todo.open .ck', '今天').click();
    await settle();
    const [date, time] = [...document.querySelectorAll<HTMLInputElement>('.pop input')];
    // Planned for today, 30 minutes estimated, and the next quarter of an hour after 10:40.
    expect([date!.value, time!.value]).toEqual([DAY, '10:45']);
    expect(document.querySelector('.pop .seg .on')!.textContent).toBe('30 分钟');
    expect(document.querySelector('.pop .slots')!.textContent).toContain('13:00');
    const scheduled = { ...reply, block: { event_id: 'b2', start: at('10:45'), end: at('11:15') } };
    table['POST /items/i2/schedule'] = { item: scheduled };
    table['GET /today'] = { date: DAY, items: [scheduled], overdue: [], events: [], free_minutes: 180, unplanned_minutes: 0 };
    document.querySelector<HTMLFormElement>('.pop form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await settle();
    expect(callsTo('POST', '/items/i2/schedule')[0]!.body).toEqual({ start: '2026-10-13T14:45:00Z', end: '2026-10-13T15:15:00Z' });
    expect(document.querySelector('.todo.open .ck')!.textContent).toBe('今天 10:45–11:15');
  });

  it('marks an item important at once', async () => {
    await open('#/items/today');
    find('.todo .tm', '回复 PR 评论').click();
    await settle();
    table['PATCH /items/i2'] = { item: { ...reply, important: true } };
    find('.todo.open .ck', '标为重要').click();
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(find('.todo.open .ck', '重要').className).toContain('hot');
    await settle();
    expect(callsTo('PATCH', '/items/i2')[0]!.body).toEqual({ important: true });
  });

  it('saves an edited title when the card closes', async () => {
    await open('#/items/today');
    find('.todo .tm', '回复 PR 评论').click();
    await settle();
    table['PATCH /items/i2'] = { item: { ...reply, title: '回复全部 PR 评论' } };
    await type(document.querySelector<HTMLTextAreaElement>('.todo.open textarea.tt')!, '回复全部 PR 评论');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await settle();
    expect(callsTo('PATCH', '/items/i2')[0]!.body).toEqual({ title: '回复全部 PR 评论' });
    expect(document.querySelector('.todo.open')).toBeNull();
  });

  it('adds a new item from the floating button, planned for today', async () => {
    await open('#/items/today');
    find('.fab', '新事项').click();
    await settle();
    const title = document.querySelector<HTMLTextAreaElement>('.todo.open textarea.tt')!;
    await type(title, '买牙膏');
    table['POST /items'] = { item: item('i9', '买牙膏', { planned_date: DAY }) };
    title.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    await settle();
    expect(callsTo('POST', '/items')[0]!.body).toEqual({ title: '买牙膏', notes: '', project_id: null, heading_id: null, planned_date: DAY });
    expect(document.querySelector('.todo.open')).toBeNull();
  });

  it('discards a new item closed without a title', async () => {
    await open('#/items/inbox');
    find('.fab', '新事项').click();
    await settle();
    expect(document.querySelector('.todo.open')).not.toBeNull();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await settle();
    expect(document.querySelector('.todo.open')).toBeNull();
    expect(callsTo('POST', '/items')).toHaveLength(0);
  });

  it('pages the inbox', async () => {
    table['GET /items'] = { items: Array.from({ length: 20 }, (_, k) => item(`in${k}`, `事项 ${k}`)), next_cursor: 'c', total: 120 };
    await open('#/items/inbox');
    const first = callsTo('GET', '/items')[0]!.query;
    expect([first.get('project_id'), first.get('limit')]).toEqual(['none', '20']);
    expect(document.querySelectorAll('.todo')).toHaveLength(20);
    expect(document.querySelector('.morebtn')!.textContent).toBe('再显示 60 件（还有 100 件）');
  });

  it('shows the plan day by day', async () => {
    await open('#/items/upcoming');
    const q = callsTo('GET', '/upcoming')[0]!.query;
    expect([q.get('from'), q.get('to')]).toEqual(['2026-10-14', '2026-10-31']);
    expect(document.querySelector('.dh')!.textContent).toBe('14明天');
    expect(text()).toContain('客户演示');
    expect(find('.morebtn', '加载').textContent).toBe('加载 11月');
  });

  it('shows the four quadrants', async () => {
    await open('#/items/matrix');
    expect(document.getElementById('list')!.className).toBe('list wide');
    expect([...document.querySelectorAll('.quad-h')].map(h => h.textContent)).toEqual([
      '马上做重要 · 紧急1 件', '安排时间做重要 · 不紧急1 件', '尽快处理掉不重要 · 紧急1 件', '有空再说不重要 · 不紧急0 件',
    ]);
    expect(document.querySelector('.quad .none')!.textContent).toBe('没有事项');
  });

  it('shows everything by project', async () => {
    await open('#/items/all');
    expect(document.querySelector('button.sec')!.textContent).toBe('Keduly 开发6');
    expect(document.querySelectorAll('.todo')).toHaveLength(5);
    expect(document.querySelector('.morebtn')!.textContent).toBe('显示其余 1 件');
  });

  it('groups the log by month and reports the total', async () => {
    table['GET /items'] = { items: [finished, { ...finished, id: 'i8', title: '写周报', completed_at: at('18:00', '2026-09-30') }], next_cursor: null, total: 2 };
    await open('#/items/done');
    expect(callsTo('GET', '/items')[0]!.query.get('status')).toBe('done');
    expect(document.querySelector('.summary')!.textContent).toBe('共 2 件，按完成时间从近到远');
    expect([...document.querySelectorAll('.sec')].map(s => s.textContent)).toEqual(['本月1', '9月1']);
    expect(document.querySelector('.end')!.textContent).toBe('已经到最早的一件了');
  });

  it('shows a project page with headings, upcoming events and the logged count', async () => {
    await open('#/items/p/p1');
    expect(document.querySelector<HTMLTextAreaElement>('.lh textarea')!.value).toBe('Keduly 开发');
    expect(document.querySelector('.summary')!.textContent).toBe('3 件未完成，其中 1 件还没排进日历；本周已专注 3 小时 7 分钟');
    expect(document.querySelector('.ev')!.textContent).toBe('今天 10:00设计评审');
    expect(document.querySelector('.sec.fold')!.textContent).toBe('同步1');
    expect(document.querySelector('.logged')!.textContent).toBe('显示 2 件已完成');
    find('.sec.fold', '同步').click();
    await settle();
    expect(document.querySelector('.sec.fold')!.className).toContain('shut');
    expect([...document.querySelectorAll('.todo .tt')].map(e => e.textContent)).toEqual(['回复 PR 评论']);
  });

  it('shows empty states instead of blank screens', async () => {
    table['GET /items'] = { items: [], next_cursor: null, total: 0 };
    table['GET /today'] = { date: DAY, items: [], overdue: [], events: [], free_minutes: 540, unplanned_minutes: 0 };
    await open('#/items/inbox');
    expect(document.querySelector('.blank b')!.textContent).toBe('收件箱是空的');
    await open('#/items/today');
    expect(document.querySelector('.blank b')!.textContent).toBe('今天没有安排');
  });
});

describe('bell and settings', () => {
  it('lists pending deletions, the other suggestions and recent activity', async () => {
    await open('#/items/today');
    find('.rbtn', '动态').click();
    await settle();
    const panel = document.querySelector('.panel')!;
    expect(panel.textContent).toContain('想删除「1:1 与王敏」');
    expect(panel.textContent).toContain('Claude Code · 王敏本周请假');
    expect(panel.textContent).toContain('1 项待定安排');
    expect(panel.textContent).toContain('新建事项「调研 FullCalendar 授权」');
    expect(panel.textContent).toContain('Claude Code · 今天 07:58');
    expect(panel.textContent).toContain('更早的动态');
    table['POST /activity/act1/undo'] = { activity: { ...activity, undone: true } };
    table['GET /activity'] = { activities: [{ ...activity, undone: true }], next_cursor: null };
    find('.panel .tb', '撤销').click();
    await settle();
    expect(document.querySelector('.p-row.undone')).not.toBeNull();
    expect(find('.panel .tb', '恢复')).toBeTruthy();
  });

  it('accepts the scheduling suggestions one by one while a deletion is waiting, then undoes them together', async () => {
    const second: Suggestion = { ...suggestion, id: 's3', title: '写招聘 JD' };
    table['GET /suggestions'] = { suggestions: [suggestion, deletion, second] };
    await open('#/items/today');
    find('.rbtn', '动态').click();
    await settle();
    table['POST /suggestions/s1/accept'] = { suggestion, activity };
    table['POST /suggestions/s3/accept'] = { suggestion: second, activity: { ...activity, id: 'act2' } };
    table['POST /activity/undo'] = { activities: [] };
    find('.panel .tb', '全部接受').click();
    await settle();
    expect(callsTo('POST', '/suggestions/accept-all')).toHaveLength(0);
    expect(callsTo('POST', '/suggestions/s1/accept')).toHaveLength(1);
    expect(callsTo('POST', '/suggestions/s3/accept')).toHaveLength(1);
    expect(document.querySelector('.hud')!.textContent).toContain('已接受 2 项安排');
    find('.hud button', '撤销').click();
    await settle();
    expect(callsTo('POST', '/activity/undo')[0]!.body).toEqual({ ids: ['act1', 'act2'] });
  });

  it('uses accept-all when no deletion is waiting', async () => {
    table['GET /suggestions'] = { suggestions: [suggestion, { ...suggestion, id: 's3', title: '写招聘 JD' }] };
    await open('#/items/today');
    find('.rbtn', '动态').click();
    await settle();
    table['POST /suggestions/accept-all'] = { accepted: 2, activity_ids: ['act1', 'act2'] };
    find('.panel .tb', '全部接受').click();
    await settle();
    expect(callsTo('POST', '/suggestions/accept-all')).toHaveLength(1);
    expect(document.querySelector('.hud')!.textContent).toContain('已接受 2 项安排');
  });

  it('opens settings with the server address and the CLI snippet', async () => {
    await open('#/items/today');
    find('.rbtn', '设置').click();
    await settle();
    const sheet = document.querySelector('.sheet')!;
    expect(sheet.textContent).toContain('me@example.com');
    expect(sheet.textContent).toContain(`${location.origin}/dav/`);
    expect(sheet.textContent).toContain(`keduly login --server ${location.origin}`);
    expect(sheet.textContent).toContain('Claude Code · MacBook');
    expect(sheet.textContent).toContain('读写，删除需确认');
    find('.sheet .seg button', '深色').click();
    expect(document.documentElement.dataset.theme).toBe('dark');
    find('.sheet .seg button', '自动').click();
    expect(document.documentElement.dataset.theme).toBeUndefined();
  });

  it('shows a new credential once', async () => {
    await open('#/items/today');
    find('.rbtn', '设置').click();
    await settle();
    find('.g-row', '新建凭证…').click();
    await settle();
    await type(document.querySelector<HTMLInputElement>('.g-form input')!, 'Codex');
    table['POST /tokens'] = { token: { id: 'k2', name: 'Codex', kind: 'agent', scope: 'write', confirm_delete: true, last_used_at: null, created_at: at('10:40'), token: 'kdl_secret' } };
    document.querySelector<HTMLFormElement>('.g-form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await settle();
    expect(callsTo('POST', '/tokens')[0]!.body).toEqual({ name: 'Codex', kind: 'agent', scope: 'write', confirm_delete: true });
    expect(document.querySelector('.secret')!.textContent).toBe('kdl_secret');
    expect(text()).toContain('只显示这一次');
  });
});

describe('focus', () => {
  const NOW = new Date(`${DAY}T10:40:00`);
  const later = (minutes: number, seconds = 0) => new Date(NOW.getTime() + minutes * 60_000 + seconds * 1000);
  const stamp = (date: Date) => date.toISOString().replace('.000Z', 'Z');
  /** What the server answers once the tomato started at 10:40 has run out. */
  const over = (): Focus => ({ ...working, state: 'over', session: { ...working.session!, completed: true }, tomatoes_today: 2, minutes_today: 50, round_done: 2, now: stamp(later(25, 1)) });
  afterEach(() => { vi.setSystemTime(NOW); });

  it('opens on the timer with the first important item of today picked', async () => {
    await open('#/focus/timer');
    expect(document.body.dataset.mode).toBe('focus');
    expect(document.body.hasAttribute('data-zen')).toBe(false);
    expect(document.querySelector('.zlab')!.textContent).toBe('第 2 个番茄');
    expect(document.querySelector('.dial b')!.textContent).toBe('25:00');
    expect(document.querySelector('.zt')!.textContent).toBe('实现 CalDAV 同步');
    expect(document.querySelector('.zs')!.textContent).toBe('Keduly 开发 · 1.5 小时 · 2/4');
    expect(document.querySelectorAll('.round .tom')).toHaveLength(4);
    expect(document.querySelectorAll('.round .tom:not(.off)')).toHaveLength(1);
    expect(document.querySelector('.round span')!.textContent).toBe('再 3 个后长休息');
    expect(document.querySelector('.zf')!.textContent).toBe('今天 1 · 25 分钟');
    // The sidebar offers today's open items, important first, then free focus.
    const choices = [...document.querySelectorAll('#side-focus .nv')].map(b => b.textContent);
    expect(choices).toEqual(['计时', '统计', '实现 CalDAV 同步2/4', '回复 PR 评论2', '写周报3', '预约体检', '自由专注']);
    expect(document.querySelector('#side-focus .nv.sel')!.textContent).toContain('实现 CalDAV 同步');
  });

  it('picks another item or free focus for the next tomato', async () => {
    await open('#/focus/timer');
    const free: Focus = { ...working, session: session('f3', at('10:40'), { item_id: null, project_id: null, title: '', completed: false }) };
    table['POST /focus/start'] = { focus: free };
    find('#side-focus .nv', '自由专注').click();
    await settle();
    expect(document.querySelector('.zt')!.textContent).toBe('自由专注');
    expect(document.querySelector('.zs')!.textContent).toBe('不记到任何事项上');
    table['GET /focus'] = { focus: free };
    find('.zb', '开始专注').click();
    await settle();
    expect(callsTo('POST', '/focus/start')[0]!.body).toEqual({});
    // Free focus has no item to tick off.
    expect([...document.querySelectorAll('.za .zb')].map(b => b.textContent)).toEqual(['放弃']);
  });

  it('starts a tomato from an item row, takes the whole window, and collapses to a capsule', async () => {
    await open('#/items/today');
    expect(find('.todo', '实现 CalDAV 同步').querySelector('.ts')!.textContent).toBe('Keduly 开发 · 1.5 小时 · 2/4');
    table['POST /focus/start'] = { focus: working };
    table['GET /focus'] = { focus: working };
    find('.todo', '回复 PR 评论').querySelector<HTMLElement>('.play')!.click();
    await settle();
    expect(callsTo('POST', '/focus/start')[0]!.body).toEqual({ item_id: 'i2' });
    expect(mod.route.route.value.mode).toBe('focus');
    expect(document.body.hasAttribute('data-zen')).toBe(true);
    expect(document.querySelector('.dial b')!.textContent).toBe('25:00');
    expect(document.querySelector('.dial .dt span')!.textContent).toBe('到 11:05');
    expect(document.querySelector('.zt')!.textContent).toBe('回复 PR 评论');
    expect([...document.querySelectorAll('.za .zb')].map(b => b.textContent)).toEqual(['放弃', '做完了']);

    // The countdown follows the clock without asking the server.
    const asked = callsTo('GET', '/focus').length;
    vi.setSystemTime(later(10));
    await mod.focus.tickFocus();
    await settle();
    expect(document.querySelector('.dial b')!.textContent).toBe('15:00');
    expect(callsTo('GET', '/focus')).toHaveLength(asked);

    find('.zback', '收起').click();
    await settle();
    expect(document.body.dataset.mode).toBe('tasks');
    expect(document.body.hasAttribute('data-zen')).toBe(false);
    expect(document.querySelector('.fcap')!.textContent).toBe('15:00回复 PR 评论');
    expect(find('.todo', '回复 PR 评论').querySelector('.fgo')!.textContent).toBe('15:00');
    expect(find('.todo', '回复 PR 评论').querySelector('.play')).toBeNull();
    document.querySelector<HTMLElement>('.fcap')!.click();
    await settle();
    expect(document.body.hasAttribute('data-zen')).toBe(true);
  });

  it('asks the server when the time runs out, and only then shows the tomato as earned', async () => {
    table['GET /focus'] = { focus: working };
    await open('#/focus/timer');
    expect(document.querySelector('.zlab')!.textContent).toBe('第 2 个番茄');

    // The device says the time is up, the server does not yet: nothing is invented.
    vi.setSystemTime(later(25, 1));
    table['GET /focus'] = { focus: { ...working, now: stamp(later(25, 1)) } };
    await mod.focus.tickFocus();
    await settle();
    expect(document.querySelector('.zlab')!.textContent).toBe('第 2 个番茄');
    expect(document.querySelector('.dial .tom')).toBeNull();

    table['GET /focus'] = { focus: over() };
    await mod.focus.tickFocus();
    await settle();
    expect(document.querySelector('.zlab')!.textContent).toBe('完成第 2 个番茄');
    expect(document.querySelector('.dial .tom')).not.toBeNull();
    expect(document.querySelectorAll('.round .tom:not(.off)')).toHaveLength(2);
    expect([...document.querySelectorAll('.za .zb')].map(b => b.textContent)).toEqual(['做完了', '再来一个', '休息 5 分钟']);

    const resting: Focus = { ...over(), state: 'rest', session: session('r1', stamp(later(25, 1)), { kind: 'rest', item_id: null, project_id: null, title: '', planned_minutes: 5, end: stamp(later(30, 1)), completed: false }) };
    table['POST /focus/rest'] = { focus: resting };
    table['GET /focus'] = { focus: resting };
    find('.zb', '休息 5 分钟').click();
    await settle();
    expect(callsTo('POST', '/focus/rest')).toHaveLength(1);
    expect(document.querySelector('.zlab')!.textContent).toBe('休息');
    expect(document.querySelector('.dial b')!.textContent).toBe('05:00');
    expect([...document.querySelectorAll('.za .zb')].map(b => b.textContent)).toEqual(['跳过休息']);
  });

  it('names free focus and files it under a project or an item, from the timer and from the records', async () => {
    const free = session('f3', at('10:40'), { item_id: null, project_id: null, title: '', completed: false });
    const on = (s: FocusSession): Focus => ({ ...working, session: s });
    const change = async (field: number, value: string) => {
      const el = document.querySelectorAll<HTMLInputElement | HTMLSelectElement>('.pop .fld')[field]!;
      el.value = value;
      el.dispatchEvent(new Event(el.tagName === 'INPUT' ? 'input' : 'change', { bubbles: true }));
      if (el.tagName === 'INPUT') el.dispatchEvent(new Event('change', { bubbles: true }));
      await settle();
    };
    table['GET /focus'] = { focus: on(free) };
    await open('#/focus/timer');
    expect(document.querySelector('.zs')!.textContent).toBe('不记到任何事项上 · 归到…');
    find('.zlink', '归到…').click();
    await settle();
    expect([...document.querySelectorAll('.pop .lbl')].map(l => l.firstChild!.textContent)).toEqual(['项目', '事项', '做了什么']);
    // Without a project the items offered are today's.
    expect([...document.querySelectorAll('.pop select')[1]!.querySelectorAll('option')].map(o => o.textContent)).toContain('实现 CalDAV 同步');

    const named = { ...free, title: '读 RFC' };
    table['PATCH /focus/sessions/f3'] = { session: named };
    table['GET /focus'] = { focus: on(named) };
    await change(2, '读 RFC');
    expect(callsTo('PATCH', '/focus/sessions/f3')[0]!.body).toEqual({ title: '读 RFC' });
    expect(document.querySelector('.zt')!.textContent).toBe('读 RFC');

    const filed = { ...named, project_id: 'p1' };
    table['PATCH /focus/sessions/f3'] = { session: filed };
    table['GET /focus'] = { focus: on(filed) };
    await change(0, 'p1');
    expect(callsTo('PATCH', '/focus/sessions/f3')[1]!.body).toEqual({ item_id: null, project_id: 'p1' });
    expect(document.querySelector('.zs')!.textContent).toContain(`${bootstrap.projects[0]!.name} · 修改`);
    expect(callsTo('GET', '/items').at(-1)!.query.get('project_id')).toBe('p1');

    // One more of the same keeps the title and the project.
    table['GET /focus'] = { focus: { ...over(), session: { ...filed, completed: true } } };
    await open('#/focus/timer');
    table['POST /focus/start'] = { focus: on(filed) };
    find('.zb', '再来一个').click();
    await settle();
    expect(callsTo('POST', '/focus/start')[0]!.body).toEqual({ title: '读 RFC', project_id: 'p1' });

    // A record in the statistics opens the same popover; on an item it has no title of its own.
    await open('#/focus/stats');
    find('.rrow', '自由专注').click();
    await settle();
    table['PATCH /focus/sessions/f9'] = { session: { ...log[1]!, item_id: 'i1', project_id: 'p1', title: '实现 CalDAV 同步' } };
    await change(1, 'i1');
    expect(callsTo('PATCH', '/focus/sessions/f9')[0]!.body).toEqual({ item_id: 'i1' });
    find('.rrow', '整理需求文档').click();
    await settle();
    expect(document.querySelectorAll('.pop .fld')).toHaveLength(2);
    // The item it is on is offered even when it is not one of the project's open items.
    expect(document.querySelectorAll<HTMLSelectElement>('.pop select')[1]!.value).toBe('i6');
  });

  it('offers the long rest when a round is complete', async () => {
    table['GET /focus'] = { focus: { ...over(), tomatoes_today: 4, round_done: 4, rest_minutes: 15 } };
    await open('#/focus/timer');
    expect(document.querySelectorAll('.round .tom:not(.off)')).toHaveLength(4);
    expect(document.querySelector('.round span')!.textContent).toBe('完成一轮，该长休息了');
    expect(find('.zb.go', '长休息 15 分钟')).not.toBeNull();
  });

  it('tells the user elsewhere in the app that a tomato was earned', async () => {
    table['GET /focus'] = { focus: working };
    await open('#/items/today');
    vi.setSystemTime(later(25, 1));
    table['GET /focus'] = { focus: over() };
    await mod.focus.tickFocus();
    await settle();
    expect(document.querySelector('.fcap')!.textContent).toBe('到点了回复 PR 评论');
    expect(document.querySelector('.hud')!.textContent).toBe('完成第 2 个番茄开始休息');
    table['POST /focus/rest'] = { focus: idle };
    find('.hud button', '开始休息').click();
    await settle();
    expect(callsTo('POST', '/focus/rest')).toHaveLength(1);
  });

  it('gives up a tomato and says what was kept', async () => {
    table['GET /focus'] = { focus: working };
    await open('#/focus/timer');
    vi.setSystemTime(later(12));
    table['POST /focus/stop'] = { focus: { ...idle, minutes_today: 37, now: stamp(later(12)) } };
    table['GET /focus'] = { focus: { ...idle, minutes_today: 37, now: stamp(later(12)) } };
    find('.zb', '放弃').click();
    await settle();
    expect(callsTo('POST', '/focus/stop')).toHaveLength(1);
    expect(document.querySelector('.hud')!.textContent).toBe('这个番茄没有完成，记了 12 分钟');
    expect(document.body.hasAttribute('data-zen')).toBe(false);
    // The item given up on stays picked.
    expect(document.querySelector('.zt')!.textContent).toBe('回复 PR 评论');
  });

  it('finishes the item from the timer by ticking it', async () => {
    table['GET /focus'] = { focus: working };
    await open('#/focus/timer');
    table['PATCH /items/i2'] = { item: { ...reply, status: 'done', completed_at: at('10:41') } };
    table['GET /focus'] = { focus: idle };
    find('.zb', '做完了').click();
    await settle();
    expect(callsTo('PATCH', '/items/i2')[0]!.body).toEqual({ status: 'done' });
    expect(callsTo('POST', '/focus/stop')).toEqual([]);
    expect(document.querySelector('.hud')!.textContent).toBe('已完成「回复 PR 评论」');
  });

  it('shows the statistics of the last seven days', async () => {
    await open('#/focus/stats');
    expect([...document.querySelectorAll('.figs > div')].map(f => f.textContent)).toEqual(['今天125 分钟', '最近 7 天208 小时 32 分钟', '日均2.9个1 小时 13 分钟', '连续专注2天每天至少一个番茄']);
    const bars = [...document.querySelectorAll<HTMLElement>('.chart .cc')];
    expect(bars.map(b => b.querySelector('em')?.textContent ?? '')).toEqual(['2', '4', '4', '3', '', '6', '1']);
    // The busiest day fills the bar's share of the chart; a day without tomatoes is an empty column.
    expect(bars[5]!.querySelector<HTMLElement>('.cst')!.style.height).toBe('84%');
    expect(bars[4]!.querySelector<HTMLElement>('.cst')!.style.height).toBe('0%');
    expect([...document.querySelectorAll('.cx span')].map(s => s.textContent)).toEqual(['周三', '周四', '周五', '周六', '周日', '周一', '今天']);
    expect([...document.querySelectorAll('.srow')].map(r => r.textContent)).toEqual(['Keduly 开发11 · 4 小时 47 分钟', '未归项目9 · 3 小时 45 分钟']);
    expect([...document.querySelectorAll('.rrow')].map(r => r.textContent)).toEqual(['09:32–09:57回复 PR 评论', '16:00–16:12自由专注未完成 · 12 分钟', '10:00–10:25整理需求文档']);
    const q = callsTo('GET', '/focus/sessions')[0]!.query;
    expect([q.get('from'), q.get('to')]).toEqual(['2026-10-07', DAY]);
  });

  it('says so when there is nothing to count yet', async () => {
    table['GET /focus/stats'] = { days: stats.days.map(d => ({ ...d, tomatoes: 0, minutes: 0, projects: [] })), projects: [], tomatoes: 0, minutes: 0, streak: 0 };
    table['GET /focus/sessions'] = { sessions: [] };
    table['GET /today'] = { date: DAY, items: [], overdue: [], events: [], free_minutes: 540, unplanned_minutes: 0 };
    table['GET /focus'] = { focus: { ...idle, tomatoes_today: 0, minutes_today: 0, round_done: 0 } };
    await open('#/focus/stats');
    expect(text()).toContain('还没有专注记录');
    // Without any item, free focus is what the timer offers.
    find('#side-focus .nv', '计时').click();
    await settle();
    expect(document.querySelector('.zlab')!.textContent).toBe('第 1 个番茄');
    expect(document.querySelector('.zt')!.textContent).toBe('自由专注');
    expect(document.querySelector('.zf')).toBeNull();
  });

  it('draws the sessions of a day beside the plan, and starts a tomato from a time block', async () => {
    await open(`#/cal/day/${DAY}`);
    // During its item's time block a session is a line beside it; a slip of a few minutes is a line too.
    // Where nothing was planned for it, it is a block of its own that says what the time went to.
    table['GET /focus/sessions'] = { sessions: [
      session('f1', at('09:32')),
      session('f5', at('15:40'), { item_id: 'i1', title: '实现 CalDAV 同步' }),
      session('f6', at('12:00'), { item_id: null, project_id: null, title: '', end: at('12:04'), completed: false }),
      session('f7', at('20:48'), { item_id: null, project_id: null, title: '等高线学习' }),
    ] };
    // Late in the evening, so that none of them is still running.
    vi.setSystemTime(later(680));
    table['GET /focus'] = { focus: { ...idle, now: stamp(later(680)) } };
    await open(`#/cal/day/${DAY}`);
    const strips = [...document.querySelectorAll<HTMLElement>('.col .fs')];
    expect(strips.map(s => s.title)).toEqual(['专注 15:40–16:05 · 实现 CalDAV 同步', '专注 12:00–12:04 · 自由专注']);
    expect(parseFloat(strips[0]!.style.top)).toBeCloseTo((15 + 40 / 60) * 58, 3);
    const blocks = [...document.querySelectorAll<HTMLElement>('.col .blk.ses')];
    expect(blocks.map(b => b.textContent)).toEqual(['回复 PR 评论09:32–09:57', '等高线学习20:48–21:13']);
    expect(parseFloat(blocks[1]!.style.top)).toBeCloseTo((20 + 48 / 60) * 58 + 1, 3);
    const q = callsTo('GET', '/focus/sessions')[0]!.query;
    expect([q.get('from'), q.get('to')]).toEqual([DAY, DAY]);

    // Clicking the block files the session.
    blocks[1]!.click();
    await settle();
    expect([...document.querySelectorAll('.col .pop .lbl')].map(l => l.firstChild!.textContent)).toEqual(['项目', '事项', '做了什么']);
    table['PATCH /focus/sessions/f7'] = { session: session('f7', at('20:48'), { item_id: null, project_id: 'p1', title: '等高线学习' }) };
    const project = document.querySelector<HTMLSelectElement>('.col .pop select')!;
    project.value = 'p1';
    project.dispatchEvent(new Event('change', { bubbles: true }));
    await settle();
    expect(callsTo('PATCH', '/focus/sessions/f7')[0]!.body).toEqual({ item_id: null, project_id: 'p1' });
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await settle();
    expect(document.querySelector('.col .pop')).toBeNull();

    find('.blk', '实现 CalDAV 同步').click();
    await settle();
    const onBlock: Focus = { ...working, session: session('f4', at('10:40'), { item_id: 'i1', title: '实现 CalDAV 同步', completed: false }) };
    table['POST /focus/start'] = { focus: onBlock };
    table['GET /focus'] = { focus: onBlock };
    find('.pop .pbtn', '开始专注').click();
    await settle();
    expect(callsTo('POST', '/focus/start')[0]!.body).toEqual({ item_id: 'i1' });
    expect(document.body.hasAttribute('data-zen')).toBe(true);
  });

  it('shows the time spent in the item card', async () => {
    await open('#/items/today');
    find('.todo', '实现 CalDAV 同步').querySelector<HTMLElement>('.tm')!.click();
    await settle();
    expect(find('.ck', '1.5 小时').textContent).toBe('1.5 小时 · 已用 50 分钟');
    expect(find('.ck.go', '开始专注')).not.toBeNull();
  });

  it('saves the length of a tomato from settings', async () => {
    await open('#/items/today');
    find('.rbtn', '设置').click();
    await settle();
    table['PATCH /me'] = { user: { ...bootstrap.user, focus_minutes: 50 } };
    const input = find('.g-row', '一个番茄').querySelector('input')!;
    input.value = '50';
    input.dispatchEvent(new Event('blur'));
    await settle();
    expect(callsTo('PATCH', '/me').map(c => c.body)).toEqual([{ focus_minutes: 50 }]);
    // Out of range: put back, not sent.
    input.value = '0';
    input.dispatchEvent(new Event('blur'));
    await settle();
    expect(callsTo('PATCH', '/me')).toHaveLength(1);
  });
});

describe('change detection', () => {
  it('refetches what is on screen when the revision moves, and not otherwise', async () => {
    await open('#/items/today');
    const before = callsTo('GET', '/today').length;
    await mod.store.poll();
    await settle();
    expect(callsTo('GET', '/today')).toHaveLength(before);
    table['GET /counts'] = { counts: { inbox: 1, today: 9, pending: 0 }, revision: 8 };
    table['GET /bootstrap'] = { ...bootstrap, counts: { inbox: 1, today: 9, pending: 0 }, revision: 8 };
    await mod.store.poll();
    await settle();
    expect(callsTo('GET', '/today')).toHaveLength(before + 1);
    expect(find('.nv', '今天').querySelector('.n')!.textContent).toBe('9');
    expect(document.querySelector('.badge')).toBeNull();
  });

  it('returns to sign-in when the session ends', async () => {
    await open('#/items/today');
    table['GET /counts'] = 401;
    await mod.store.poll();
    await settle();
    expect(document.querySelector('.auth-card')).not.toBeNull();
  });
});

describe('time zone', () => {
  const away = () => ({ ...bootstrap, user: { ...bootstrap.user, timezone: 'Asia/Shanghai' } });
  afterEach(() => localStorage.clear());

  it('asks before changing an account whose zone differs from the device, and shows times in the account zone meanwhile', async () => {
    table['GET /bootstrap'] = away();
    await open(`#/cal/day/${DAY}`);
    expect(text()).toContain('这台设备的时区是 America/New_York，你的账号用的是 Asia/Shanghai');
    // Nothing was changed on the user's behalf.
    expect(callsTo('PATCH', '/me')).toEqual([]);
    // 10:40 in New York is 22:40 in Shanghai.
    expect(document.querySelector('.nowcap')!.textContent).toBe('22:40');
  });

  it('switches the account to the device zone when asked to', async () => {
    table['GET /bootstrap'] = away();
    table['PATCH /me'] = { user: bootstrap.user };
    await open(`#/cal/day/${DAY}`);
    find('.sheet.ask button', '改用 America/New_York').click();
    await settle();
    expect(callsTo('PATCH', '/me').map(c => c.body)).toEqual([{ timezone: 'America/New_York' }]);
  });

  it('follows the device without asking only when the user opted in', async () => {
    table['GET /bootstrap'] = { ...bootstrap, user: { ...bootstrap.user, timezone: 'Asia/Shanghai', timezone_auto: true } };
    table['PATCH /me'] = { user: bootstrap.user };
    await open(`#/cal/day/${DAY}`);
    expect(callsTo('PATCH', '/me').map(c => c.body)).toEqual([{ timezone: 'America/New_York' }]);
    expect(document.querySelector('.sheet.ask')).toBeNull();
  });

  // Last: the answer is remembered for the rest of the page's life.
  it('stops asking on this device once the user keeps the account zone', async () => {
    table['GET /bootstrap'] = away();
    await open(`#/cal/day/${DAY}`);
    find('.sheet.ask button', '保持 Asia/Shanghai').click();
    await settle();
    expect(document.querySelector('.sheet.ask')).toBeNull();
    expect(callsTo('PATCH', '/me')).toEqual([]);
    await mod.store.enter();
    await settle();
    expect(document.querySelector('.sheet.ask')).toBeNull();
  });
});
