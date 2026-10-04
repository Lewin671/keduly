// Every request the web app makes goes through this module, one function per endpoint of docs/api.md.
import type {
  Activity, Area, Bootstrap, CalEvent, Config, Counts, EventWrite, FreeSlot, Heading, Item, ItemFilters, ItemPage, ItemWrite,
  NewToken, OverviewProject, Project, ProjectColor, ProjectDetail, Quadrant, QuadrantKey, Suggestion, TodayView, Token,
  UpcomingDay, User,
} from './types';

const BASE = '/api/v1';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type Query = Record<string, string | number | undefined | null>;

function withQuery(path: string, query?: Query): string {
  if (!query) return path;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null) params.set(key, String(value));
  }
  const text = params.toString();
  return text ? `${path}?${text}` : path;
}

async function errorFrom(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: { code?: unknown; message?: unknown } };
    if (body.error && typeof body.error.code === 'string') {
      return new ApiError(res.status, body.error.code, typeof body.error.message === 'string' ? body.error.message : res.statusText);
    }
  } catch {
    // Not JSON: a proxy or the network answered instead of the server.
  }
  return new ApiError(res.status, 'http', res.statusText || `HTTP ${res.status}`);
}

/** Sends one request. Resolves to the parsed body, or `undefined` for an empty response. */
export async function request<T>(method: string, path: string, options: { query?: Query; body?: unknown } = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (method !== 'GET') headers['X-Keduly-Request'] = '1';
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  let res: Response;
  try {
    res = await fetch(withQuery(BASE + path, options.query), {
      method,
      headers,
      credentials: 'same-origin',
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
  } catch {
    throw new ApiError(0, 'network', 'network error');
  }
  if (!res.ok) throw await errorFrom(res);
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  if (!text) return undefined as T;
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new ApiError(res.status, 'bad_response', 'response is not JSON');
  }
}

const get = <T>(path: string, query?: Query) => request<T>('GET', path, { query });
const post = <T>(path: string, body?: unknown) => request<T>('POST', path, { body });
const patch = <T>(path: string, body: unknown) => request<T>('PATCH', path, { body });
const del = <T = void>(path: string) => request<T>('DELETE', path);
const id = encodeURIComponent;

/* ---------- service and account ---------- */

export const getConfig = () => get<Config>('/config');

export const register = (body: { email: string; password: string; name: string; timezone: string }) =>
  post<{ user: User }>('/auth/register', body).then(r => r.user);
export const login = (body: { email: string; password: string }) => post<{ user: User }>('/auth/login', body).then(r => r.user);
export const logout = () => post<void>('/auth/logout');
export const getMe = () => get<{ user: User }>('/me').then(r => r.user);
export const updateMe = (body: Partial<Pick<User, 'name' | 'timezone' | 'work_start' | 'work_end'>>) =>
  patch<{ user: User }>('/me', body).then(r => r.user);
export const changePassword = (current: string, next: string) => post<void>('/me/password', { current, new: next });

/* ---------- bootstrap and change detection ---------- */

export const getBootstrap = () => get<Bootstrap>('/bootstrap');
export const getCounts = () => get<{ counts: Counts; revision: number }>('/counts');

/* ---------- areas, projects and headings ---------- */

export const createArea = (name: string) => post<{ area: Area }>('/areas', { name }).then(r => r.area);
export const updateArea = (areaId: string, body: Partial<Pick<Area, 'name' | 'position'>>) =>
  patch<{ area: Area }>(`/areas/${id(areaId)}`, body).then(r => r.area);
export const deleteArea = (areaId: string) => del(`/areas/${id(areaId)}`);

export const createProject = (body: { name: string; color?: ProjectColor; area_id?: string | null; notes?: string }) =>
  post<{ project: Project }>('/projects', body).then(r => r.project);
export const getProject = (projectId: string) => get<ProjectDetail>(`/projects/${id(projectId)}`);
export const updateProject = (
  projectId: string,
  body: Partial<Pick<Project, 'name' | 'color' | 'area_id' | 'notes' | 'position' | 'archived'>>,
) => patch<{ project: Project }>(`/projects/${id(projectId)}`, body).then(r => r.project);
export const deleteProject = (projectId: string) => del(`/projects/${id(projectId)}`);

export const createHeading = (projectId: string, name: string) =>
  post<{ heading: Heading }>(`/projects/${id(projectId)}/headings`, { name }).then(r => r.heading);
export const updateHeading = (headingId: string, body: Partial<Pick<Heading, 'name' | 'position'>>) =>
  patch<{ heading: Heading }>(`/headings/${id(headingId)}`, body).then(r => r.heading);
export const deleteHeading = (headingId: string) => del(`/headings/${id(headingId)}`);

/* ---------- items ---------- */

export const listItems = (filters: ItemFilters & { limit?: number; cursor?: string | null }) => get<ItemPage>('/items', { ...filters });
export const createItem = (body: ItemWrite & { title: string }) => post<{ item: Item }>('/items', body).then(r => r.item);
export const getItem = (itemId: string) => get<{ item: Item }>(`/items/${id(itemId)}`).then(r => r.item);
export const updateItem = (itemId: string, body: ItemWrite & { status?: Item['status'] }) =>
  patch<{ item: Item }>(`/items/${id(itemId)}`, body).then(r => r.item);
export const deleteItem = (itemId: string) => del(`/items/${id(itemId)}`);
export const scheduleItem = (itemId: string, start: string, end: string) =>
  post<{ item: Item }>(`/items/${id(itemId)}/schedule`, { start, end }).then(r => r.item);
export const unscheduleItem = (itemId: string) => del<{ item: Item }>(`/items/${id(itemId)}/schedule`).then(r => r.item);

const PAGE_MAX = 200;

/** The first `count` items of a list, following cursors when that is more than one page. */
export async function listItemsUpTo(filters: ItemFilters, count: number): Promise<ItemPage> {
  const items: Item[] = [];
  let cursor: string | null = null;
  let total = 0;
  do {
    const page: ItemPage = await listItems({ ...filters, limit: Math.min(count - items.length, PAGE_MAX), cursor });
    items.push(...page.items);
    total = page.total;
    cursor = page.next_cursor;
  } while (cursor && items.length < count);
  return { items, total, next_cursor: cursor };
}

/* ---------- views ---------- */

export const getToday = () => get<TodayView>('/today');
export const getUpcoming = (from: string, to: string) => get<{ days: UpcomingDay[] }>('/upcoming', { from, to }).then(r => r.days);
export const getOverview = () => get<{ projects: OverviewProject[] }>('/overview').then(r => r.projects);
export const getMatrix = () => get<{ quadrants: Record<QuadrantKey, Quadrant> }>('/matrix').then(r => r.quadrants);

/* ---------- events ---------- */

export const listEvents = (from: string, to: string) => get<{ events: CalEvent[] }>('/events', { from, to }).then(r => r.events);
export const createEvent = (body: EventWrite & { title: string }) => post<{ event: CalEvent }>('/events', body).then(r => r.event);
export const getEvent = (eventId: string) => get<{ event: CalEvent }>(`/events/${id(eventId)}`).then(r => r.event);
export const updateEvent = (eventId: string, body: EventWrite) => patch<{ event: CalEvent }>(`/events/${id(eventId)}`, body).then(r => r.event);
export const deleteEvent = (eventId: string) => del(`/events/${id(eventId)}`);
export const getHeat = (year: number) => get<{ days: Record<string, number> }>('/calendar/heat', { year }).then(r => r.days);
export const getFree = (date: string, duration: number) => get<{ slots: FreeSlot[] }>('/free', { date, duration }).then(r => r.slots);

/* ---------- suggestions ---------- */

export const listSuggestions = (status: 'pending' | 'accepted' | 'rejected' | 'any' = 'pending') =>
  get<{ suggestions: Suggestion[] }>('/suggestions', { status }).then(r => r.suggestions);
export const acceptSuggestion = (suggestionId: string) =>
  post<{ suggestion: Suggestion; activity: Activity }>(`/suggestions/${id(suggestionId)}/accept`);
export const rejectSuggestion = (suggestionId: string) =>
  post<{ suggestion: Suggestion }>(`/suggestions/${id(suggestionId)}/reject`).then(r => r.suggestion);
export const acceptAllSuggestions = () => post<{ accepted: number; activity_ids: string[] }>('/suggestions/accept-all');

/* ---------- activity ---------- */

export const listActivity = (cursor?: string | null, limit = 20) =>
  get<{ activities: Activity[]; next_cursor: string | null }>('/activity', { limit, cursor });
export const undoActivity = (activityId: string) => post<{ activity: Activity }>(`/activity/${id(activityId)}/undo`).then(r => r.activity);
export const redoActivity = (activityId: string) => post<{ activity: Activity }>(`/activity/${id(activityId)}/redo`).then(r => r.activity);
export const undoActivities = (ids: string[]) => post<{ activities: Activity[] }>('/activity/undo', { ids }).then(r => r.activities);

/* ---------- tokens ---------- */

export const listTokens = () => get<{ tokens: Token[] }>('/tokens').then(r => r.tokens);
export const createToken = (body: { name: string; kind: Token['kind']; scope?: Token['scope']; confirm_delete?: boolean }) =>
  post<{ token: NewToken }>('/tokens', body).then(r => r.token);
export const deleteToken = (tokenId: string) => del(`/tokens/${id(tokenId)}`);
