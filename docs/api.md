# HTTP API

The web app and the CLI talk to the server through this JSON API. It is the contract between
the backend and the frontend: change it here first, then in the code.

## Conventions

| Topic | Rule |
|---|---|
| Base path | `/api/v1` |
| Format | JSON request and response bodies, UTF-8 |
| IDs | Opaque strings (16 lowercase base32 characters). Never parse them |
| Timestamps | RFC 3339 in UTC, e.g. `2026-10-13T07:30:00Z` |
| Dates | `YYYY-MM-DD`, interpreted in the user's time zone |
| Times of day | `HH:MM`, 24-hour |
| Durations | Integer minutes |
| Absent values | `null`, never an empty string |
| Unknown fields | Rejected with `400 invalid_request` |

### Authentication

| Client | Mechanism |
|---|---|
| Web app | Session cookie `keduly_session` (HttpOnly, SameSite=Lax, Secure over HTTPS). Every request that is not `GET` must also send the header `X-Keduly-Request: 1`; requests without it are rejected with `403 csrf` |
| CLI and agents | `Authorization: Bearer <token>`. Tokens are created in Settings and start with `kdl_` |
| CalDAV clients | HTTP Basic with the account email and an app password (a token of kind `caldav`). See [CalDAV](#caldav) |

A token has a `scope`: `read` may only call `GET` endpoints, `write` may call everything except
account, token and session management, which always require a session cookie.

### Errors

Every error has the same shape and an HTTP status that matches it:

```json
{ "error": { "code": "not_found", "message": "item not found" } }
```

| Status | Codes |
|---|---|
| 400 | `invalid_request` (malformed JSON, unknown field, failed validation; `message` names the field) |
| 401 | `unauthenticated` |
| 403 | `forbidden` (token scope too narrow), `csrf`, `registration_closed` |
| 404 | `not_found` (also returned for objects owned by another user) |
| 409 | `conflict` (e.g. email already registered, suggestion already decided) |
| 429 | `rate_limited` |
| 500 | `internal` |

### Pagination

List endpoints that can grow without bound take `limit` (default 50, maximum 200) and `cursor`.
They return `next_cursor`, which is `null` on the last page, and `total`, the size of the whole
result ignoring pagination. Cursors are opaque.

### Dry run

Every `POST`, `PATCH` and `DELETE` on items, events and suggestions accepts `?dry_run=1`.
The server validates and computes the result but writes nothing, and responds with the normal
body plus `"dry_run": true`.

### Actors

Every write is attributed to an actor, which the activity log shows:

```json
{ "kind": "user" | "agent" | "caldav" | "system", "name": "Claude Code · MacBook" }
```

`kind` is `user` for the web session, `agent` for bearer tokens (the name is the token name) and
`caldav` for CalDAV clients (the name is the app password name).

## Objects

### User

```json
{
  "id": "…", "email": "me@example.com", "name": "Me",
  "timezone": "Asia/Shanghai", "timezone_auto": true,
  "work_start": "09:00", "work_end": "18:00",
  "created_at": "…"
}
```

`timezone` decides what "today" is, how dates are read, and which wall-clock times the web app
shows. While `timezone_auto` is true (the default) the web app keeps `timezone` equal to the zone
of the device it runs on; set it to false to keep a zone chosen by hand.

### Area

A group of projects in the sidebar, such as "Work" or "Personal".

```json
{ "id": "…", "name": "工作", "position": 0 }
```

### Project

A project is both a list of items and a calendar: events and items belong to at most one project
and take its colour.

```json
{
  "id": "…", "area_id": "…" | null, "name": "Keduly 开发",
  "color": "blue", "notes": "",
  "position": 0, "archived": false,
  "open_count": 7, "done_count": 3,
  "created_at": "…", "updated_at": "…"
}
```

`color` is one of `blue`, `indigo`, `orange`, `teal`, `green`, `pink`, `purple`, `brown`.

### Heading

A titled section inside a project.

```json
{ "id": "…", "project_id": "…", "name": "同步", "position": 0 }
```

### Item

Something to do. An item with no project is in the inbox.

```json
{
  "id": "…", "project_id": "…" | null, "heading_id": "…" | null,
  "title": "实现 CalDAV 同步", "notes": "",
  "estimate_minutes": 90 | null,
  "planned_date": "2026-10-13" | null,
  "evening": false,
  "due_date": "2026-10-13" | null, "due_time": "18:00" | null,
  "important": false,
  "status": "open" | "done", "completed_at": "…" | null,
  "position": 0,
  "block": { "event_id": "…", "start": "…", "end": "…" } | null,
  "suggestion": { "id": "…", "start": "…", "end": "…", "reason": "…", "actor": { … } } | null,
  "created_by": { … }, "created_at": "…", "updated_at": "…"
}
```

| Field | Meaning |
|---|---|
| `planned_date` | The day the user intends to do it. Items planned for today or earlier and still open show in Today |
| `evening` | Show under "This evening" in Today |
| `due_date`, `due_time` | The deadline. `due_time` requires `due_date` |
| `important` | The user's mark. Urgency is never stored: an open item is urgent when `due_date` is tomorrow or earlier |
| `block` | The confirmed time block on the calendar: the next one that has not ended, or else the most recent one. Scheduling an item sets `planned_date` to the block's day |
| `suggestion` | A pending suggestion to schedule this item, if any |

### Event

Something on the calendar. A time block for an item is an event with `item_id` set.

```json
{
  "id": "…", "project_id": "…" | null, "item_id": "…" | null,
  "title": "设计评审", "notes": "", "location": "",
  "all_day": false,
  "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z",
  "start_date": null, "end_date": null,
  "rrule": null, "recurring": false, "instance": null,
  "status": "confirmed" | "tentative" | "leaving",
  "suggestion_id": null,
  "item_done": null,
  "readonly": false,
  "created_by": { … }, "updated_at": "…"
}
```

| Field | Meaning |
|---|---|
| `all_day` | When true, `start_date` and `end_date` are set (both inclusive) and `start`/`end` are `null` |
| `rrule` | The iCalendar recurrence rule of the series, e.g. `FREQ=WEEKLY;BYDAY=MO`. Only present on events that came from a CalDAV client or were created with one |
| `recurring`, `instance` | Listing events expands a series into instances. Each instance has the series `id`, `recurring: true` and `instance` set to that occurrence's original start |
| `status` | `confirmed` is a normal event. `tentative` does not exist yet: it is what a pending suggestion would create. `leaving` is a real event that a pending suggestion would move or delete |
| `suggestion_id` | Set when `status` is `tentative` or `leaving` |
| `item_done` | For time blocks, whether the item is done; otherwise `null` |
| `readonly` | True for instances of recurring events: they can be changed from a CalDAV client, not from the web app |

### Suggestion

A change proposed by an agent that takes effect only when the user accepts it.

```json
{
  "id": "…", "status": "pending" | "accepted" | "rejected",
  "kind": "schedule_item" | "create_item" | "create_event" | "move_event" | "delete_event" | "delete_item",
  "actor": { … }, "reason": "今天 18:00 截止，这是下午第一个 1 小时空档。",
  "title": "写周报",
  "item_id": "…" | null, "event_id": "…" | null,
  "start": "…" | null, "end": "…" | null,
  "item": { … } | null, "event": { … } | null,
  "created_at": "…", "decided_at": "…" | null
}
```

| Kind | Required input | Effect when accepted |
|---|---|---|
| `schedule_item` | `item_id`, `start`, `end` | Creates the item's time block |
| `create_item` | `item` (fields of a new item), optional `start`, `end` | Creates the item, and its time block when a slot is given |
| `create_event` | `event` (fields of a new event) | Creates the event |
| `move_event` | `event_id`, `start`, `end` | Moves the event |
| `delete_event` | `event_id` | Deletes the event |
| `delete_item` | `item_id` | Deletes the item |

`title` is the title of the affected item or event, filled in by the server for display.

### Activity

One entry per change, newest first.

```json
{
  "id": "…", "actor": { … },
  "action": "item.create",
  "summary": "新建事项「调研 FullCalendar 授权」",
  "reason": "…" | null,
  "undoable": true, "undone": false,
  "created_at": "…"
}
```

`summary` is written by the server in Chinese and is ready to display.

### Token

```json
{
  "id": "…", "name": "Claude Code · MacBook",
  "kind": "agent" | "caldav",
  "scope": "read" | "write",
  "confirm_delete": true,
  "last_used_at": "…" | null, "created_at": "…"
}
```

The secret is returned once, in the `token` field of the creation response.

## Endpoints

Unless stated otherwise, a successful response is `200` with the object under its own name,
e.g. `{ "item": { … } }`, and lists are under the plural, e.g. `{ "items": [ … ] }`.
`DELETE` returns `204` with no body.

### Service

| Method and path | Auth | Description |
|---|---|---|
| `GET /config` | none | `{ "registration": "open" \| "closed", "version": "…" }` |
| `GET /healthz` (not under `/api/v1`) | none | `200 ok` |

### Account

| Method and path | Body | Response |
|---|---|---|
| `POST /auth/register` | `email`, `password` (at least 8 characters), `name`, `timezone` | `201 { user }`, sets the session cookie. `403 registration_closed` when sign-up is off |
| `POST /auth/login` | `email`, `password` | `{ user }`, sets the session cookie. `401 unauthenticated` on a wrong email or password |
| `POST /auth/logout` | | `204` |
| `GET /me` | | `{ user }` |
| `PATCH /me` | any of `name`, `timezone`, `timezone_auto`, `work_start`, `work_end` | `{ user }` |
| `POST /me/password` | `current`, `new` | `204`. Ends every other session |

A new account starts with no projects.

### Bootstrap and change detection

| Method and path | Response |
|---|---|
| `GET /bootstrap` | `{ user, areas, projects, headings, counts, revision }` |
| `GET /counts` | `{ counts, revision }` |

`counts` is `{ "inbox": 2, "today": 6, "pending": 4 }`: open inbox items, open items in Today
(including overdue ones) and pending suggestions.

`revision` is an integer that increases on every write to the user's data, whoever made it. The
web app polls `GET /counts` every 20 seconds and reloads what it shows when `revision` changes.

### Areas, projects and headings

| Method and path | Body | Notes |
|---|---|---|
| `POST /areas` | `name` | |
| `PATCH /areas/{id}` | `name`, `position` | |
| `DELETE /areas/{id}` | | Its projects stay and lose their area |
| `POST /projects` | `name`, optional `color`, `area_id`, `notes` | `color` defaults to the least used one |
| `GET /projects/{id}` | | `{ project, headings, upcoming_events, unplanned_count }`. `upcoming_events` is the project's next 3 events from now, time blocks excluded |
| `PATCH /projects/{id}` | any of `name`, `color`, `area_id`, `notes`, `position`, `archived` | |
| `DELETE /projects/{id}` | | Also deletes its headings, items and events |
| `POST /projects/{id}/headings` | `name` | |
| `PATCH /headings/{id}` | `name`, `position` | |
| `DELETE /headings/{id}` | | Its items stay in the project without a heading |

### Items

| Method and path | Body or query | Response |
|---|---|---|
| `GET /items` | filters below, `limit`, `cursor` | `{ items, next_cursor, total }` |
| `POST /items` | `title`, optional `project_id`, `heading_id`, `notes`, `estimate_minutes`, `planned_date`, `evening`, `due_date`, `due_time`, `important` | `201 { item }` |
| `GET /items/{id}` | | `{ item }` |
| `PATCH /items/{id}` | any writable field, and `status` | `{ item }` |
| `DELETE /items/{id}` | | `204`, or `202 { suggestion }` when the caller is a token with `confirm_delete` |
| `POST /items/{id}/schedule` | `start`, `end` | `{ item }`. Creates the time block or moves the existing one |
| `DELETE /items/{id}/schedule` | | `{ item }`. Removes the time block |

Setting `status` to `done` records `completed_at`; setting it back to `open` clears it.

Filters for `GET /items`, combined with AND:

| Parameter | Values |
|---|---|
| `status` | `open` (default), `done`, `any` |
| `project_id` | a project ID, or `none` for the inbox |
| `heading_id` | a heading ID, or `none` |
| `quadrant` | `do` (important, urgent), `plan` (important, not urgent), `quick` (not important, urgent), `later` (neither). Open items only |
| `q` | Case-insensitive substring of the title or notes |

Open items are ordered by `position`, then creation time. Done items are ordered by
`completed_at`, newest first.

### Views

These endpoints assemble what one screen needs in a single request.

| Method and path | Response |
|---|---|
| `GET /today` | `{ date, items, overdue, events, free_minutes, unplanned_minutes }` |
| `GET /upcoming?from=&to=` | `{ days: [ { date, events, items } ] }` |
| `GET /overview` | `{ projects: [ { project_id, total, items } ] }` |
| `GET /matrix` | `{ quadrants: { do, plan, quick, later } }` |

`GET /today`

| Field | Meaning |
|---|---|
| `items` | Open items with `planned_date` today or earlier, plus items completed today. Important items first |
| `overdue` | Open items whose `due_date` is before today and that are not already in `items` |
| `events` | Today's events, time blocks excluded, all-day ones first |
| `free_minutes` | Minutes between `work_start` and `work_end` not covered by any event or time block (pending suggestions count as covered). Overlapping events are counted once |
| `unplanned_minutes` | Sum of `estimate_minutes` over items in `items` that are open, not `evening`, and have neither a time block nor a pending suggestion |

`GET /upcoming` takes two dates (inclusive, at most 62 days apart) and returns only days that have
something. `items` are open items planned for that day, or due that day when they have no planned
date. `events` exclude time blocks.

`GET /overview` returns every project that has open items, each with its first 5 open items and
the `total` number of open items. Load the rest with `GET /items?project_id=…`.

`GET /matrix` returns, per quadrant, `{ total, unplanned, items }` with the first 6 items.
`unplanned` counts items that have neither a time block nor a pending suggestion. Load the rest
with `GET /items?quadrant=…`.

### Events

| Method and path | Body or query | Response |
|---|---|---|
| `GET /events?from=&to=` | two timestamps, at most 400 days apart | `{ events }` |
| `POST /events` | `title`, `start`, `end`, or `all_day: true` with `start_date`, `end_date`; optional `project_id`, `notes`, `location` | `201 { event }` |
| `GET /events/{id}` | | `{ event }` |
| `PATCH /events/{id}` | any writable field | `{ event }` |
| `DELETE /events/{id}` | | `204`, or `202 { suggestion }` when the caller is a token with `confirm_delete` |
| `GET /calendar/heat?year=2026` | | `{ days: { "2026-10-13": 7, … } }` |
| `GET /free?date=&duration=` | a date and a number of minutes | `{ slots: [ { start, end } ] }` |

`GET /events` returns everything that overlaps the range, sorted by start:

- events and time blocks, with recurring series expanded into instances;
- for each pending suggestion that touches the range, a `tentative` entry for what it would
  create (for `schedule_item`, `create_item` with a slot, `create_event` and the new position of
  `move_event`), and the affected event marked `leaving` (for `move_event` and `delete_event`).

`GET /calendar/heat` counts, per day in the user's time zone, the events and time blocks on that
day. Days with nothing are omitted.

`GET /free` returns the gaps of at least `duration` minutes between `work_start` and `work_end`
on that day.

### Suggestions

| Method and path | Body | Response |
|---|---|---|
| `GET /suggestions?status=pending` | `status` is `pending` (default), `accepted`, `rejected` or `any` | `{ suggestions }` |
| `POST /suggestions` | `kind`, `reason`, and the inputs that kind requires | `201 { suggestion }` |
| `POST /suggestions/{id}/accept` | | `{ suggestion, activity }` |
| `POST /suggestions/{id}/reject` | | `{ suggestion }` |
| `POST /suggestions/accept-all` | | `{ accepted, activity_ids }` |

Deciding a suggestion requires a session cookie: an agent cannot accept its own proposals.
A suggestion that can no longer be applied (its item or event is gone) is rejected with
`409 conflict` on accept and is then marked `rejected`.

### Activity

| Method and path | Body | Response |
|---|---|---|
| `GET /activity` | `limit` (default 20), `cursor` | `{ activities, next_cursor }` |
| `POST /activity/{id}/undo` | | `{ activity }` |
| `POST /activity/{id}/redo` | | `{ activity }` |
| `POST /activity/undo` | `ids` | `{ activities }`. Undoes several entries, newest first |

Undo restores the affected objects to their state before the change; redo applies the change
again. Both fail with `409 conflict` when the objects have changed since in a way that makes the
result ambiguous.

### Tokens

Session cookie only.

| Method and path | Body | Response |
|---|---|---|
| `GET /tokens` | | `{ tokens }` |
| `POST /tokens` | `name`, `kind`, optional `scope` (default `write`), `confirm_delete` (default `true`) | `201 { token }` with the secret in `token.token` |
| `DELETE /tokens/{id}` | | `204` |

## CalDAV

The CalDAV endpoint lets the calendar apps on iPhone, Mac, Android (through DAVx⁵) and others
show and edit the same data.

| Topic | Rule |
|---|---|
| Root | `/dav/`. `/.well-known/caldav` redirects to it |
| Login | HTTP Basic: the account email and an app password created in Settings |
| Principal | `/dav/principals/{user-id}/` |
| Calendar home | `/dav/calendars/{user-id}/` |
| Calendars | One per project, at `/dav/calendars/{user-id}/{project-id}/`, with the project's name and colour. Events without a project are in `/dav/calendars/{user-id}/inbox/` |
| Objects | `VEVENT` only. One resource per event, named `{event-id}.ics`. Time blocks appear as ordinary events |
| Writes | Creating, changing and deleting events is supported. Creating or deleting calendars is not: projects are managed in the web app |
| Change detection | Each object has an ETag; each calendar has a CTag (`getctag`) that changes whenever anything in it does |

Events created by a CalDAV client keep their original iCalendar text, so properties this server
does not understand survive a round trip.
