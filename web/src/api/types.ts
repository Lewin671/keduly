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
  work_start: string;
  work_end: string;
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
