// Objects of the HTTP API, as documented in docs/api.md.

export type ProjectColor = 'blue' | 'indigo' | 'orange' | 'teal' | 'green' | 'pink' | 'purple' | 'brown';

export interface Actor {
  kind: 'user' | 'agent' | 'caldav' | 'system';
  name: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  timezone: string;
  timezone_auto: boolean;
  work_start: string;
  work_end: string;
  /** The length of one tomato, of the rest after it, and of the rest after every `round_size`-th tomato of the day. */
  focus_minutes: number;
  rest_minutes: number;
  long_rest_minutes: number;
  round_size: number;
  created_at: string;
}

export interface Area {
  id: string;
  name: string;
  position: number;
}

export interface Project {
  id: string;
  area_id: string | null;
  name: string;
  color: ProjectColor;
  notes: string;
  position: number;
  archived: boolean;
  open_count: number;
  done_count: number;
  created_at: string;
  updated_at: string;
}

export interface Heading {
  id: string;
  project_id: string;
  name: string;
  position: number;
}

export interface ItemBlock {
  event_id: string;
  start: string;
  end: string;
}

export interface ItemSuggestion {
  id: string;
  start: string;
  end: string;
  reason: string;
  actor: Actor;
}

/** Time spent on an item with the focus timer. A session still running is not counted. */
export interface ItemFocus {
  tomatoes: number;
  minutes: number;
}

export interface Item {
  id: string;
  project_id: string | null;
  heading_id: string | null;
  title: string;
  notes: string;
  estimate_minutes: number | null;
  planned_date: string | null;
  evening: boolean;
  due_date: string | null;
  due_time: string | null;
  important: boolean;
  status: 'open' | 'done';
  completed_at: string | null;
  position: number;
  block: ItemBlock | null;
  suggestion: ItemSuggestion | null;
  focus: ItemFocus;
  created_by: Actor;
  created_at: string;
  updated_at: string;
}

export type EventStatus = 'confirmed' | 'tentative' | 'leaving';

export interface CalEvent {
  id: string;
  project_id: string | null;
  item_id: string | null;
  title: string;
  notes: string;
  location: string;
  all_day: boolean;
  start: string | null;
  end: string | null;
  start_date: string | null;
  end_date: string | null;
  rrule: string | null;
  recurring: boolean;
  instance: string | null;
  status: EventStatus;
  suggestion_id: string | null;
  item_done: boolean | null;
  readonly: boolean;
  created_by: Actor;
  updated_at: string;
}

export type SuggestionKind = 'schedule_item' | 'create_item' | 'create_event' | 'move_event' | 'delete_event' | 'delete_item';

export interface Suggestion {
  id: string;
  status: 'pending' | 'accepted' | 'rejected';
  kind: SuggestionKind;
  actor: Actor;
  reason: string;
  title: string;
  item_id: string | null;
  event_id: string | null;
  start: string | null;
  end: string | null;
  item: Partial<Item> | null;
  event: Partial<CalEvent> | null;
  created_at: string;
  decided_at: string | null;
}

export interface Activity {
  id: string;
  actor: Actor;
  action: string;
  summary: string;
  reason: string | null;
  undoable: boolean;
  undone: boolean;
  created_at: string;
}

export interface Token {
  id: string;
  name: string;
  kind: 'agent' | 'caldav';
  scope: 'read' | 'write';
  confirm_delete: boolean;
  last_used_at: string | null;
  created_at: string;
}

/** The creation response is the only place the secret appears. */
export interface NewToken extends Token {
  token: string;
}

export interface Counts {
  inbox: number;
  today: number;
  pending: number;
}

/** A device asking to be signed in, as shown to the user who may approve it. */
export interface LoginRequest {
  id: string;
  /** The asking browser and system, e.g. "Chrome · Mac"; empty when unrecognised. */
  device: string;
  expires_at: string;
}

export interface Config {
  registration: 'open' | 'closed';
  version: string;
}

export interface Bootstrap {
  user: User;
  areas: Area[];
  projects: Project[];
  headings: Heading[];
  counts: Counts;
  revision: number;
}

export interface ItemPage {
  items: Item[];
  next_cursor: string | null;
  total: number;
}

export interface ProjectDetail {
  project: Project;
  headings: Heading[];
  upcoming_events: CalEvent[];
  unplanned_count: number;
  /** Focus time on the project's items since Monday. */
  focus_week_minutes: number;
}

export interface TodayView {
  date: string;
  items: Item[];
  overdue: Item[];
  events: CalEvent[];
  free_minutes: number;
  unplanned_minutes: number;
}

export interface UpcomingDay {
  date: string;
  events: CalEvent[];
  items: Item[];
}

export interface OverviewProject {
  project_id: string;
  total: number;
  items: Item[];
}

export type QuadrantKey = 'do' | 'plan' | 'quick' | 'later';

export interface Quadrant {
  total: number;
  unplanned: number;
  items: Item[];
}

export interface FreeSlot {
  start: string;
  end: string;
}

export interface ItemFilters {
  status?: 'open' | 'done' | 'any';
  project_id?: string;
  heading_id?: string;
  quadrant?: QuadrantKey;
  q?: string;
}

export type ItemWrite = Partial<
  Pick<Item, 'title' | 'project_id' | 'heading_id' | 'notes' | 'estimate_minutes' | 'planned_date' | 'evening' | 'due_date' | 'due_time' | 'important'>
>;

export type EventWrite = Partial<Pick<CalEvent, 'title' | 'project_id' | 'notes' | 'location' | 'all_day' | 'start' | 'end' | 'start_date' | 'end_date'>>;

/** One stretch of the focus timer: a tomato being worked on, or a rest. */
export interface FocusSession {
  id: string;
  kind: 'work' | 'rest';
  /** `null` is free focus. */
  item_id: string | null;
  /** The item's project; free focus has its own. */
  project_id: string | null;
  /** The item's title; for free focus what the user called it, or empty. */
  title: string;
  start: string;
  /** The planned end while running; the moment it was given up otherwise. */
  end: string;
  planned_minutes: number;
  /** Ran its full length and its end has passed. A completed work session is one tomato. */
  completed: boolean;
  created_by: Actor;
}

/** The state of the user's one timer. */
/** The title and the project that free focus may have. */
export type FreeFocus = Partial<Pick<FocusSession, 'title' | 'project_id'>>;
/** `item_id` puts the session on an item; `title` and `project_id` are for free focus only. */
export type SessionWrite = Partial<Pick<FocusSession, 'item_id' | 'title' | 'project_id'>>;

export interface Focus {
  state: 'idle' | 'work' | 'over' | 'rest';
  session: FocusSession | null;
  tomatoes_today: number;
  minutes_today: number;
  round_size: number;
  round_done: number;
  /** The length of the rest that would start now. */
  rest_minutes: number;
  /** The server's clock. */
  now: string;
}

export interface FocusAmount {
  project_id: string | null;
  tomatoes: number;
  minutes: number;
}

export interface FocusDay {
  date: string;
  tomatoes: number;
  minutes: number;
  projects: FocusAmount[];
}

/** The last 7 days, today included. */
export interface FocusStats {
  days: FocusDay[];
  projects: FocusAmount[];
  tomatoes: number;
  minutes: number;
  streak: number;
}
